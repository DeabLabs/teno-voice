package main

import (
	"log"
	"net/http"

	"com.deablabs.teno-voice/internal/discord"
	"github.com/go-chi/chi"
)

func main() {
	// TODO Initialize your Discord connection

	// Set up the router
	router := chi.NewRouter()
	router.Post("/join", discord.JoinVoiceCall)
	router.Post("/leave", discord.LeaveVoiceCall)

	// Start the REST API server
	log.Println("Starting REST API server on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("Error starting REST API server: %v", err)
	}
}
