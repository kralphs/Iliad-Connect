package handlers

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
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

	client := &http.Client{}

	body, err := ioutil.ReadAll(r.Body)
	w.Write(body)
	json.Unmarshal(body, &payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println(payload)

	resp, err := client.Get(payload.Data.Link)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	body, err = ioutil.ReadAll(resp.Body)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(body))
}

func pushHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/push" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

}
