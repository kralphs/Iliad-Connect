package handlers

import (
	"bytes"
	"encoding/base64"
	"log"
	"net/http"
	"parser"
	"time"

	"golang.org/x/oauth2"
	gmail "google.golang.org/api/gmail/v1"
)

func googlePush(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/email/google/push" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	log.Println("Getting User")
	user, err := Auth.GetUserByEmail(r.Context(), "kevin.b.c.ralphs@gmail.com")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println("Getting Token")
	docToken, err := firestoreClient.Collection("users").Doc(user.UID).Collection("tokens").Doc("email").Get(r.Context())
	if err != nil {
		http.Error(w, "Error retrieving email authorization token", http.StatusInternalServerError)
		return
	}

	log.Println("Token retrieved")
	token := new(oauth2.Token)
	log.Println("Mapping data to token")
	mapToken := docToken.Data()
	log.Println(mapToken)
	token.AccessToken = mapToken["AccessToken"].(string)
	token.RefreshToken = mapToken["RefreshToken"].(string)
	token.TokenType = mapToken["TokenType"].(string)
	token.Expiry = mapToken["Expiry"].(time.Time)

	log.Println("Getting oauth client")
	googleClient := googleOAuthConfig.Client(r.Context(), token)

	log.Println("Starting gmail service")
	gmailService, err := gmail.New(googleClient)
	if err != nil {
		log.Println("Failed to create Gmail service")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	getCall := gmailService.Users.Messages.Get("me", "167850a584ce7690")
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

	log.Println(link)

	// TODO: Send link and matter_id to begin document download
	return
}
