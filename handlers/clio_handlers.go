package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"parser"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

type pdf struct {
	Name    string
	Content []byte
	URL     string
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {

	type Payload struct {
		Data struct {
			Link     string `json:"link"`
			MatterID int    `json:"matterID"`
		} `json:"data"`
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

	chErrors := make(chan error)
	var wg sync.WaitGroup

	for _, file := range files {
		wg.Add(1)
		go func(file pdf) {
			defer wg.Done()
			uploadFile(clioClient, payload.Data.MatterID, file, chErrors)
		}(file)
	}

	go func() {
		for err := range chErrors {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}()

	wg.Wait()

	w.Write([]byte("Successful"))
}

func pushHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/push" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
}

func collectFiles(link string) ([]pdf, error) {
	files := []pdf{}

	client := &http.Client{}

	resp, err := client.Get(link)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Single file case
	if resp.Header.Get("Content-Type") == "application/pdf" {
		disposition := strings.SplitAfter(resp.Header.Get("Content-Disposition"), "filename=")
		name := strings.Trim(disposition[1], "\"")
		file, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		files = append(files, pdf{Name: name, Content: file, URL: link})
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
	chFiles := make(chan pdf)
	chErrors := make(chan error)
	var wg sync.WaitGroup

	for _, ref := range urls {
		wg.Add(1)
		go func(url string) {
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

func uploadFile(client *http.Client, matterID int, file pdf, ch chan error) {
	type Inquiry struct {
		Data struct {
			Name   string `json:"name"`
			Parent struct {
				ID   int    `json:"id"`
				Type string `json:"type"`
			} `json:"parent"`
		} `json:"data"`
	}

	type Bucket struct {
		Data struct {
			ID                    int `json:"id"`
			LatestDocumentVersion struct {
				UUID       string `json:"uuid"`
				PutURL     string `json:"put_url"`
				PutHeaders []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"put_headers"`
			} `json:"latest_document_version"`
		} `json:"data"`
	}

	type Uploaded struct {
		Data struct {
			UUID          string `json:"uuid"`
			FullyUploaded string `json:"fully_uploaded"`
		} `json:"data"`
	}
	// Get bucket
	var inq Inquiry
	inq.Data.Name = file.Name
	inq.Data.Parent.ID = matterID
	inq.Data.Parent.Type = "Matter"
	jsonInq, err := json.Marshal(inq)
	if err != nil {
		ch <- err
		return
	}

	params := url.Values{}
	params.Add("fields", "id,latest_document_version{uuid,put_url,put_headers}")
	u := "https://app.clio.com/api/v4/documents.json"

	post, err := http.NewRequest("POST", u, bytes.NewBuffer(jsonInq))
	post.URL.RawQuery = params.Encode()
	post.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(post)
	if err != nil {
		ch <- err
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		ch <- errors.New(resp.Status)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	var bucket Bucket
	err = json.Unmarshal(body, &bucket)
	if err != nil {
		ch <- err
		return
	}

	// Upload File

	put, err := http.NewRequest("PUT", bucket.Data.LatestDocumentVersion.PutURL, bytes.NewBuffer(file.Content))
	for _, header := range bucket.Data.LatestDocumentVersion.PutHeaders {
		put.Header.Add(header.Name, header.Value)
	}

	newClient := &http.Client{}
	resp2, err := newClient.Do(put)
	if err != nil {
		log.Println(err.Error())
		ch <- err
	}
	defer resp2.Body.Close()
	body, err = ioutil.ReadAll(resp2.Body)

	if resp2.StatusCode != 200 {
		ch <- errors.New(resp2.Status)
		return
	}

	// Mark completed
	var uploaded Uploaded
	uploaded.Data.UUID = bucket.Data.LatestDocumentVersion.UUID
	uploaded.Data.FullyUploaded = "true"
	jsonUploaded, err := json.Marshal(uploaded)

	params = url.Values{}
	params.Add("fields", "id,latest_document_version{fully_uploaded}")
	params.Add("external_property_name", "link")
	params.Add("external_property_value", file.URL)
	u = "https://app.clio.com/api/v4/documents/" + strconv.Itoa(bucket.Data.ID)

	patch, err := http.NewRequest("PATCH", u, bytes.NewBuffer(jsonUploaded))
	patch.Header.Add("Content-Type", "application/json")
	patch.URL.RawQuery = params.Encode()

	resp3, err := client.Do(patch)
	if err != nil {
		ch <- err
		return
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != 200 {
		ch <- errors.New(resp3.Status)
		return
	}

	return
}

func downloadFile(client *http.Client, u string, ch chan pdf, chErrors chan error) {

	resp, err := client.Get(u)
	if err != nil {
		chErrors <- err
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") == "application/pdf" {
		disposition := strings.SplitAfter(resp.Header.Get("Content-Disposition"), "filename=")
		name := strings.Trim(disposition[1], "\"")
		file, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			chErrors <- err
		}
		ch <- pdf{Name: name, Content: file, URL: u}
	} else {
		chErrors <- errors.New("Invalid or stale link")
	}
}
