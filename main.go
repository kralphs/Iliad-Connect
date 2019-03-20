package main

import (
	// "fmt"
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
	if err := server.ListenAndServeTLS("localhost.crt", "localhost.key"); err != http.ErrServerClosed {
		//if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Printf("%v", err)
	} else {
		log.Println("Server closed!")
	}
}
