package handlers

import (
	"context"
	"errors"
	"log"

	"github.com/gomodule/redigo/redis"
)

func processEmail(subject string, body string) {
	return
}

func isOdyssey(ctx context.Context, subject string) (bool, error) {
	cache := pool.Get()
	defer cache.Close()

	var subjects [][]byte
	subjects, err := redis.ByteSlices(cache.Do("LRANGE", "subjects", 0, -1))
	if err != nil {
		return false, errors.New("Error fetching subject list from cache")
	}

	if len(subjects) == 0 {
		docSubjects, err := firestoreClient.Collection("lists").Doc("subjects").Get(ctx)
		if err != nil {
			return false, errors.New("Error fetching subject list from database")
		}
		mapSubjects := docSubjects.Data()
		log.Println(mapSubjects)
		strSubjects := mapSubjects["list"].([]string)
		subjects = make([][]byte, len(strSubjects))

		for i, subject := range strSubjects {
			subjects[i] = []byte(subject)
			_, err := cache.Do("RPUSH", "subjects", string(subject))
			if err != nil {
				return false, errors.New("Error pushing subject onto list in cache")
			}
		}
	}

	log.Println(string(subjects[0]))
	log.Println(len(subjects))

	return true, nil
}
