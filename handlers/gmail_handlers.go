package handlers

import (
	"bytes"
	"encoding/base64"
	"iliad-connect/parser"
	"log"
	"net/http"
	"strconv"

	gmail "google.golang.org/api/gmail/v1"
)

func googlePush(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/email/google/push" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	email := "kevin.b.c.ralphs@gmail.com"
	messageID := "167850a584ce7690"

	user, err := Auth.GetUserByEmail(r.Context(), email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	emailClient, err := getOauthClient(r.Context(), user.UID, "email")

	gmailService, err := gmail.New(emailClient)
	if err != nil {
		log.Println("Failed to create Gmail service")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	getCall := gmailService.Users.Messages.Get("me", messageID)
	getCall = getCall.Context(r.Context())
	message, err := getCall.Do()
	if err != nil {
		log.Println("Failed to retrive message")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var subject string
	for _, header := range message.Payload.Headers {
		if header.Name == "Subject" {
			subject = header.Value
			break
		}
	}

	caseNumber, err := getCaseNumber(r.Context(), subject)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	labels, err := getOdysseyLabels(r.Context(), gmailService, user.UID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	log.Println(caseNumber)
	log.Println(labels)

	// TODO: Check if attached matter. If not, change label to "Odyssey AR"
	clioClient, err := getOauthClient(r.Context(), user.UID, "clio")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	matterID, err := getMatterID(r.Context(), clioClient, caseNumber, user.UID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	log.Println(matterID)

	var body []byte
	for _, part := range message.Payload.Parts {
		if part.MimeType == "text/html" {
			body, err = base64.URLEncoding.DecodeString(part.Body.Data)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			break
		}
	}

	urls := parser.GetUrls(bytes.NewReader([]byte(body)), true)

	whiteList, err := getWhiteList(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	link := findLink(urls, whiteList)

	w.Write([]byte("Matter ID: " + strconv.Itoa(matterID) + ", CaseNumber: " + caseNumber + ", Link: " + link))

	// TODO: Send link and matter_id to begin document download
	return
}
