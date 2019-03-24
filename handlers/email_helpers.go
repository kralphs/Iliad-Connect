package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"iliad-connect/parser"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/gomodule/redigo/redis"
	gmail "google.golang.org/api/gmail/v1"
)

func addLabel(ctx context.Context, srv *gmail.Service, messageID, labelID string) error {
	var messageRequest gmail.ModifyMessageRequest
	messageRequest.AddLabelIds = []string{labelID}
	addCall := srv.Users.Messages.Modify("me", messageID, &messageRequest)
	_, err := addCall.Do()
	if err != nil {
		return err
	}

	return nil
}

func processEmail(ctx context.Context, srv *gmail.Service, client *http.Client, uid string, messageID string) error {

	getCall := srv.Users.Messages.Get("me", messageID)
	getCall = getCall.Context(ctx)
	message, err := getCall.Do()
	if err != nil {
		log.Println("Failed to retrieve message")
		return err
	}
	var subject string
	for _, header := range message.Payload.Headers {
		if header.Name == "Subject" {
			subject = header.Value
			break
		}
	}

	caseNumber, err := getCaseNumber(ctx, subject)
	if err != nil {
		log.Println("Failed to get case number")
		return err
	}

	if caseNumber == "" {
		return nil
	}

	labels, err := getOdysseyLabels(ctx, srv, uid)
	if err != nil {
		log.Println("Failed to retrieve labels")
		return err
	}

	matterID, err := getMatterID(ctx, client, caseNumber, uid)
	if err != nil {
		log.Println("Error retrieving matter ID")
		return err
	}
	if matterID == 0 {
		err = addLabel(ctx, srv, messageID, labels["Odyssey AR"])
		if err != nil {
			log.Println("Error adding label")
			return err
		}
		return nil
	}

	var body []byte
	for _, part := range message.Payload.Parts {
		if part.MimeType == "text/html" {
			body, err = base64.URLEncoding.DecodeString(part.Body.Data)
			if err != nil {
				addLabel(ctx, srv, messageID, labels["Odyssey AR"])
				return err
			}
			break
		}
	}

	urls := parser.GetUrls(bytes.NewReader([]byte(body)), true)

	whiteList, err := getWhiteList(ctx)
	if err != nil {
		log.Println("Error fetching white list")
		addLabel(ctx, srv, messageID, labels["Odyssey AR"])
		return err
	}

	link := findLink(urls, whiteList)

	err = processLink(ctx, client, matterID, link)
	if err != nil {
		log.Println("Error processing link")
		addLabel(ctx, srv, messageID, labels["Odyssey AR"])
		return err
	}

	addLabel(ctx, srv, messageID, labels["Odyssey"])

	return nil
}

func getCaseNumber(ctx context.Context, subject string) (string, error) {
	cache := pool.Get()
	defer cache.Close()

	var subjects [][]byte
	subjects, err := redis.ByteSlices(cache.Do("LRANGE", "subjects", 0, -1))
	if err != nil {
		return "", errors.New("Error fetching subject list from cache")
	}

	if len(subjects) == 0 {
		docSubjects, err := firestoreClient.Collection("lists").Doc("subjects").Get(ctx)
		if err != nil {
			return "", errors.New("Error fetching subject list from database")
		}
		mapSubjects := docSubjects.Data()
		strSubjects := mapSubjects["list"].([]interface{})
		subjects = make([][]byte, len(strSubjects))

		for i, subject := range strSubjects {
			subjects[i] = []byte(subject.(string))
			_, err := cache.Do("RPUSH", "subjects", string(subject.(string)))
			if err != nil {
				return "", errors.New("Error pushing subject onto list in cache")
			}
		}
	}

	for _, sub := range subjects {
		reg, err := regexp.Compile(string(sub))
		if err != nil {
			return "", err
		}
		result := reg.FindSubmatch([]byte(subject))
		if len(result) != 0 {
			return string(result[1]), nil
		}
	}

	return "", nil
}

func getOdysseyLabels(ctx context.Context, srv *gmail.Service, uid string) (map[string]string, error) {
	result := make(map[string]string)

	cache := pool.Get()
	defer cache.Close()

	// Check cache for label. If not present, get from Gmail. If still not present, create it.
	labelID, err := redis.String(cache.Do("GET", uid+":Odyssey"))
	if err != nil && err != redis.ErrNil {
		return nil, err
	}

	labelARID, err := redis.String(cache.Do("GET", uid+":Osyssey AR"))
	if err != nil && err != redis.ErrNil {
		return nil, err

	}

	if labelID != "" && labelARID != "" {
		result["Odyssey"] = labelID
		result["Odyssey AR"] = labelARID
		return result, nil
	}

	listCall := srv.Users.Labels.List("me")
	listCall = listCall.Context(ctx)

	labels, err := listCall.Do()
	if err != nil {
		return nil, err
	}

	for _, label := range labels.Labels {
		switch label.Name {
		case "Odyssey":
			labelID = label.Id
			_, err := cache.Do("SET", uid+":Odyssey", labelID)
			if err != nil {
				return nil, err
			}
		case "Odyssey AR":
			labelARID = label.Id
			_, err := cache.Do("SET", uid+":Odyssey AR", labelID)
			if err != nil {
				return nil, err
			}
		}
	}

	var wg sync.WaitGroup

	if labelID == "" {
		wg.Add(1)
		go func() {
			label := new(gmail.Label)
			label.Name = "Odyssey"
			label.MessageListVisibility = "show"
			label.LabelListVisibility = "labelShow"
			createCall := srv.Users.Labels.Create("me", label)
			createCall = createCall.Context(ctx)
			newLabel, err := createCall.Do()
			if err != nil {
				// TODO: Add error channel
			}
			labelID = newLabel.Id
			_, err = cache.Do("SET", uid+":Odyssey", labelID)
			wg.Done()
		}()
	}

	if labelARID == "" {
		wg.Add(1)
		go func() {
			label := new(gmail.Label)
			label.Name = "Odyssey AR"
			label.MessageListVisibility = "show"
			label.LabelListVisibility = "labelShow"
			createCall := srv.Users.Labels.Create("me", label)
			createCall = createCall.Context(ctx)
			newLabel, err := createCall.Do()
			if err != nil {
				// TODO: Add error channel
			}
			labelARID = newLabel.Id
			_, err = cache.Do("SET", uid+":Odyssey AR", labelARID)
			wg.Done()
		}()
	}

	wg.Wait()

	result["Odyssey"] = labelID
	result["Odyssey AR"] = labelARID

	return result, nil
}

func getWhiteList(ctx context.Context) ([]string, error) {

	cache := pool.Get()
	defer cache.Close()

	domains, err := redis.Strings(cache.Do("LRANGE", "domains", 0, -1))
	if err != nil && err != redis.ErrNil {
		return nil, err
	}

	if err == redis.ErrNil {
		docDomains, err := firestoreClient.Collection("lists").Doc("domains").Get(ctx)
		if err != nil {
			return nil, err
		}
		mapDomains := docDomains.Data()
		strDomains := mapDomains["list"].([]interface{})
		domains = make([]string, len(strDomains))
		for i, str := range strDomains {
			domains[i] = fmt.Sprint(str)
			cache.Do("LPUSH", "domains", fmt.Sprint(str))
		}
	}

	return domains, nil
}

func findLink(urls, domains []string) string {
	for _, url := range urls {
		for _, domain := range domains {
			if strings.Index(url, domain) != -1 {
				return url
			}
		}
	}
	return ""
}

func getEmail(ctx context.Context, srv *gmail.Service, uid string) (string, error) {

	docUser, err := firestoreClient.Collection("users").Doc(uid).Get(ctx)
	if err != nil {
		return "", err
	}

	mapUser := docUser.Data()
	provider := mapUser["email"]

	switch provider {
	case "google":
		getCall := srv.Users.GetProfile("me")
		getCall.Context(ctx)
		profile, err := getCall.Do()
		if err != nil {
			return "", err
		}

		return profile.EmailAddress, nil
	}

	return "", errors.New("Could not retrieve email from uid")
}
