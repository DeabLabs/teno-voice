package main

import (
	"context"
	"net/http"

	"com.deablabs.teno-voice/internal/auth"
	"com.deablabs.teno-voice/internal/calls"
	Config "com.deablabs.teno-voice/internal/config"
	"com.deablabs.teno-voice/internal/deps"
	"com.deablabs.teno-voice/internal/discord"
	"com.deablabs.teno-voice/internal/redis"
	"github.com/disgoorg/log"
	"github.com/go-chi/chi"
)

func main() {
	log.Info("starting up")

	token := Config.Environment.DiscordToken

	log.Info("Waiting for discord client to be ready")

	client, closeClient, err := discord.NewClient(context.Background(), token)

	if err != nil {
		log.Fatal("error creating discord client: ", err)
	}

	defer closeClient()

	redisAddr := Config.Environment.Redis

	redisClient, redisCloseClient := redis.NewClient(context.Background(), redisAddr)

	defer redisCloseClient()

	// create a new instance of the Deps struct
	// We pass this struct into the handlers so they can access the discord client
	// and kill signal
	dependencies := &deps.Deps{DiscordClient: &client, RedisClient: redisClient}

	// Set up the router, connected to discord functionality
	router := chi.NewRouter()
	router.Use(auth.ApiKeyAuthMiddleware(Config.Environment.ApiKey))
	router.Post("/join", calls.JoinVoiceCall(dependencies))
	router.Post("/leave", calls.LeaveVoiceCall(dependencies))
	router.Get("/transcript/{guild_id}", calls.TranscriptSSEHandler(dependencies))
	router.Get("/config/{guild_id}", calls.ConfigResponder(dependencies))

	// Start the REST API server
	log.Info("Starting REST API server on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("Error starting REST API server: %v", err)
	}
}
