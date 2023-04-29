package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi"
)

func joinVoiceCall(w http.ResponseWriter, r *http.Request) {
	// TODO Join the voice call
	log.Println("Joining voice call...")
}

func leaveVoiceCall(w http.ResponseWriter, r *http.Request) {
	// TODO Leave the voice call
	log.Println("Leaving voice call...")
}

func main() {
	// TODO Initialize your Discord connection

	// Set up the router
	router := chi.NewRouter()
	router.Post("/join", joinVoiceCall)
	router.Post("/leave", leaveVoiceCall)

	// Start the REST API server
	log.Println("Starting REST API server on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("Error starting REST API server: %v", err)
	}
}
