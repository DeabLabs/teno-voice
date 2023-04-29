package discord

import (
	"log"
	"net/http"
)

func JoinVoiceCall(w http.ResponseWriter, r *http.Request) {
	// TODO Join the voice call
	log.Println("Joining voice call...")
}

func LeaveVoiceCall(w http.ResponseWriter, r *http.Request) {
	// TODO Leave the voice call
	log.Println("Leaving voice call...")
}
