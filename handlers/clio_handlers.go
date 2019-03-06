package handlers

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"parser"
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

	w.WriteHeader(http.StatusOK)
	for _, file := range files {
		log.Println(file)
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

	if resp.Header.Get("Content-Type") == "application/pdf" {
		file, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
		return files, nil
	}

	urls := parser.GetUrls(resp.Body, false)
	if len(urls) == 0 {
		return nil, errors.New("Invalid or stale link")
	}

	base, err := url.Parse(link)
	if err != nil {
		return nil, err
	}

	for _, ref := range urls {

		urlRef, err := url.Parse(ref)
		if err != nil {
			return nil, err
		}
		u := base.ResolveReference(urlRef)

		res, err := client.Get(u.String())
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		if res.Header.Get("Content-Type") == "application/pdf" {
			file, err := ioutil.ReadAll(res.Body)
			if err != nil {
				return nil, err
			}
			files = append(files, file)
		} else {
			return nil, errors.New("Invalid or stale link")
		}
	}

	return files, nil
}
