package handlers

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"parser"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Upload Handler")

	type Data struct {
		MatterID string `json:"matterID"`
		Link     string `json:"link"`
	}

	type Payload struct {
		Data `json:"data"`
	}

	// Redirects back to root if URL misrouted
	if r.URL.Path != "/upload" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	var payload Payload

	body, err := ioutil.ReadAll(r.Body)

	json.Unmarshal(body, &payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	files, err := collectFiles(payload.Data.Link)
	if err != nil {
		// Invalid or Stale link
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	user, err := Auth.GetUserByEmail(r.Context(), "kevin.b.c.ralphs@gmail.com")

	docToken, err := firestoreClient.Collection("users").Doc(user.UID).Collection("tokens").Doc("clio").Get(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	token := new(oauth2.Token)
	mapToken := docToken.Data()
	token.AccessToken = mapToken["AccessToken"].(string)
	token.RefreshToken = mapToken["RefreshToken"].(string)
	token.TokenType = mapToken["TokenType"].(string)
	token.Expiry = mapToken["Expiry"].(time.Time)

	clioClient := clioOAuthConfig.Client(r.Context(), token)

	for _, file := range files {
		uploadFile(clioClient, payload.Data.MatterID, payload.Data.Link, file)
	}
}

func pushHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/push" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

}

func collectFiles(link string) ([][]byte, error) {
	files := [][]byte{}

	client := &http.Client{}

	resp, err := client.Get(link)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Single file case
	if resp.Header.Get("Content-Type") == "application/pdf" {
		file, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
		return files, nil
	}

	// Multiple files or invalid link
	urls := parser.GetUrls(resp.Body, false)
	if len(urls) == 0 {
		return nil, errors.New("Invalid or stale link")
	}

	// Convert relative paths to full URLs
	base, err := url.Parse(link)
	if err != nil {
		return nil, err
	}

	for i, ref := range urls {
		urlRef, err := url.Parse(ref)
		if err != nil {
			return nil, err
		}
		u := base.ResolveReference(urlRef)
		urls[i] = u.String()
	}

	// Attempt fetching files from each link
	chFiles := make(chan []byte)
	chErrors := make(chan error)
	var wg sync.WaitGroup

	for _, ref := range urls {
		wg.Add(1)
		go func(url string) {
			log.Println(url)
			downloadFile(client, url, chFiles, chErrors)
			wg.Done()
		}(ref)
	}

	for i := 0; i < len(urls); i++ {
		select {
		case file := <-chFiles:
			files = append(files, file)
		case err := <-chErrors:
			return nil, err
		}
	}
	wg.Wait()
	return files, nil
}

func uploadFile(client *http.Client, matterID string, link string, file []byte) error {
	log.Println("Uploaded a File")
	return nil
}

func downloadFile(client *http.Client, u string, ch chan []byte, chErrors chan error) {

	resp, err := client.Get(u)
	if err != nil {
		chErrors <- err
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") == "application/pdf" {
		file, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			chErrors <- err
		}
		ch <- file
	} else {
		chErrors <- errors.New("Invalid or stale link")
	}
	log.Println("File downloaded")
}
