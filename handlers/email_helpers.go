package handlers

import (
	"context"
	"errors"
	"log"
	"regexp"

	"github.com/gomodule/redigo/redis"
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

func getOdysseyLabels() (map[string]string, error) {
	return make(map[string]string), nil
}
