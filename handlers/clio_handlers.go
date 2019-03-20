package handlers

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

func uploadHandler(w http.ResponseWriter, r *http.Request) {

	type Payload struct {
		Data struct {
			Link     string `json:"link"`
			MatterID int    `json:"matterID"`
			Email    string `json:"email"`
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

	user, err := Auth.GetUserByEmail(r.Context(), payload.Data.Email)

	clioClient, err := getOauthClient(r.Context(), user.UID, "clio")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = processLink(r.Context(), clioClient, payload.Data.MatterID, payload.Data.Link)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Successful"))
}
