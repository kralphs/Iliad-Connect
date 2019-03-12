package handlers

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
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

func googleWatch(w http.ResponseWriter, r *http.Request) {
	return
}
