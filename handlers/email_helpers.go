package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"

	"github.com/gomodule/redigo/redis"
	gmail "google.golang.org/api/gmail/v1"
)

func processEmail(subject string, body string) {
	return
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
		log.Println(mapSubjects["list"])
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

	log.Println(subject)
	for _, sub := range subjects {
		reg, err := regexp.Compile(string(sub))
		if err != nil {
			return "", err
		}
		log.Println(string(sub))
		result := reg.FindSubmatch([]byte(subject))
		if len(result) != 0 {
			return string(result[1]), nil
		}
	}

	return "", nil
}

func getOdysseyLabels(ctx context.Context, srv *gmail.Service, uid string) (map[string]string, error) {
	log.Println("Getting Labels")
	result := make(map[string]string)

	cache := pool.Get()
	defer cache.Close()

	// Check cache for label. If not present, get from Gmail. If still not present, create it.
	labelID, err := redis.String(cache.Do("GET", uid+":Odyssey"))
	if err != nil && err != redis.ErrNil {
		return nil, err
	}

	log.Println("From cache: Odyssey - " + labelID)

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

	log.Println(len(labels.Labels))
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
