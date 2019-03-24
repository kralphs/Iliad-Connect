package main

import (
	"fmt"
	"iliad-connect/handlers"
	"log"
	"net/http"
	"os"
)

func main() {

	log.Println(os.Getenv("PORT"))

	server := &http.Server{
		Addr:    fmt.Sprintf(":" + os.Getenv("PORT")),
		Handler: handlers.New(),
	}

	log.Printf("Starting HTTP Server. Listening at %q", server.Addr)

	if _, ok := os.LookupEnv("LOCAL_SERVER"); ok {
		if err := server.ListenAndServeTLS("localhost.crt", "localhost.key"); err != http.ErrServerClosed {
			log.Printf("%v", err)
		} else {
			log.Println("Server closed!")
		}
	} else {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("%v", err)
		} else {
			log.Println("Server closed!")
		}
	}
}
