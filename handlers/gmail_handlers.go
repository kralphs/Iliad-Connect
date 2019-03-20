package handlers

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	gmail "google.golang.org/api/gmail/v1"
)

func googlePush(w http.ResponseWriter, r *http.Request) {

	type Push struct {
		Message struct {
			Data        string    `json:"data"`
			MessageID   string    `json:"messageId"`
			PublishTime time.Time `json:"publishTime"`
		} `json:"message"`
		Subscription string `json:"subscription"`
	}

	type Message struct {
		EmailAddress string `json:"emailAddress"`
		HistoryID    uint64 `json:"historyId"`
	}

	if r.URL.Path != "/email/google/push" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Process request body
	log.Println("Processing Push")

	body, err := ioutil.ReadAll(r.Body)
	var push Push
	json.Unmarshal(body, &push)
	rawMessage, err := base64.URLEncoding.DecodeString(push.Message.Data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var message Message
	json.Unmarshal([]byte(rawMessage), &message)

	email := message.EmailAddress

	// Retrieve UID associated with this watch; note this may not be
	// the same email used for account creation.

	docEmail, err := firestoreClient.Collection("googleWatch").Doc(email).Get(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	mapEmail := docEmail.Data()
	uid, ok := mapEmail["UID"].(string)
	if !ok {
		http.Error(w, "Error retrienving UID from email", http.StatusInternalServerError)
		return
	}

	// Set up needed http clients

	emailClient, err := getOauthClient(r.Context(), uid, "email")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	gmailService, err := gmail.New(emailClient)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// Make history request

	// First get last HistoryID from database
	docWatch, err := firestoreClient.Collection("googleWatch").Doc(email).Get(r.Context())
	if err != nil {
		log.Printf("Error getting watch record: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	mapWatch := docWatch.Data()
	startHistoryID := uint64(mapWatch["StartHistoryID"].(int64))

	historyCall := gmailService.Users.History.List("me")
	historyCall = historyCall.Context(r.Context())
	historyCall = historyCall.StartHistoryId(startHistoryID)
	historyCall = historyCall.HistoryTypes("messageAdded")
	resp, err := historyCall.Do()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println(message.HistoryID)
	messageIDs := []string{}
	for _, history := range resp.History {
		for _, message := range history.MessagesAdded {
			if len(message.Message.LabelIds) != 0 {
				messageIDs = append(messageIDs, message.Message.Id)
			}
		}
	}

	// TODO: Add error channels?

	clioClient, err := getOauthClient(r.Context(), uid, "clio")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var wg sync.WaitGroup

	for _, messageID := range messageIDs {
		go func(messageID string) {
			wg.Add(1)
			err = processEmail(r.Context(), gmailService, clioClient, uid, messageID)
			if err != nil {
				log.Println(err.Error())
				log.Println(messageID)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				wg.Done()
				return
			}
			wg.Done()
		}(messageID)
	}
	log.Println("Waiting...")
	wg.Wait()

	_, err = firestoreClient.Collection("googleWatch").Doc(email).Update(r.Context(), []firestore.Update{
		{
			Path:  "StartHistoryID",
			Value: int64(resp.HistoryId),
		},
	})
	if err != nil {
		log.Printf("Error updating history: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte("Successful"))

	return
}

func googleWatch(w http.ResponseWriter, r *http.Request) {

	type Watch struct {
		UID            string `json:"UID"`
		StartHistoryID int64  `json:"startHistoryID"`
	}

	user, err := checkSession(r)
	if err != nil {
		log.Printf("error retrieving user information: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	client, err := getOauthClient(r.Context(), user.UID, "email")
	if err != nil {
		log.Printf("error retrieving oauth client: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	srv, err := gmail.New(client)
	if err != nil {
		log.Printf("error retrieving gmail service: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Set Gmail watch
	var watchRequest gmail.WatchRequest
	watchRequest.TopicName = "projects/iliad-connect-227218/topics/iliadPush"
	watchCall := srv.Users.Watch("me", &watchRequest)
	watchCall = watchCall.Context(r.Context())
	watchResponse, err := watchCall.Do()
	if err != nil {
		log.Printf("error setting watch: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Update database fields
	email, err := getEmail(r.Context(), srv, user.UID)
	if err != nil {
		log.Printf("error retrieving email: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var watch Watch
	watch.UID = user.UID
	watch.StartHistoryID = int64(watchResponse.HistoryId)

	_, err = firestoreClient.Collection("googleWatch").Doc(email).Set(r.Context(), watch)
	if err != nil {
		log.Printf("error adding watch to database: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = firestoreClient.Collection("users").Doc(user.UID).Update(r.Context(), []firestore.Update{
		{
			Path:  "scanning",
			Value: true,
		},
	})
	if err != nil {
		log.Printf("error adding scanning flag: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)

	return

}

func googleStop(w http.ResponseWriter, r *http.Request) {
	user, err := checkSession(r)
	if err != nil {
		log.Printf("error retrieving user information: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	client, err := getOauthClient(r.Context(), user.UID, "email")
	if err != nil {
		log.Printf("error retrieving oauth client: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	srv, err := gmail.New(client)
	if err != nil {
		log.Printf("error retrieving gmail service: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	email, err := getEmail(r.Context(), srv, user.UID)
	if err != nil {
		log.Printf("error retrieving email: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	stopCall := srv.Users.Stop("me")
	stopCall = stopCall.Context(r.Context())
	err = stopCall.Do()
	if err != nil {
		log.Printf("Error stopping watch: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	_, err = firestoreClient.Collection("users").Doc(user.UID).Update(r.Context(), []firestore.Update{
		{
			Path:  "scanning",
			Value: firestore.Delete,
		},
	})
	if err != nil {
		log.Printf("Error removing scanning flag: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	_, err = firestoreClient.Collection("googleWatch").Doc(email).Delete(r.Context())
	if err != nil {
		log.Printf("Error removing watch lookup from database: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)

	return
}
