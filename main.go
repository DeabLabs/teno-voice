package main

import (
	"context"
	"net/http"

	"com.deablabs.teno-voice/internal/auth"
	"com.deablabs.teno-voice/internal/calls"
	Config "com.deablabs.teno-voice/internal/config"
	"com.deablabs.teno-voice/internal/deps"
	"com.deablabs.teno-voice/internal/llm"
	"com.deablabs.teno-voice/internal/redis"
	texttospeech "com.deablabs.teno-voice/internal/textToSpeech"
	"github.com/disgoorg/log"
	"github.com/go-chi/chi"
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func main() {
	// log.SetLevel(log.LevelTrace)
	// log.SetFlags(log.LstdFlags | log.Llongfile)

	validate = validator.New()

	validate.RegisterValidation("LLMConfigValidation", llm.LLMConfigValidation)
	validate.RegisterValidation("TTSConfigValidation", texttospeech.TTSConfigValidation)

	log.Info("starting up")

	redisAddr := Config.Environment.Redis

	redisClient, redisCloseClient := redis.NewClient(context.Background(), redisAddr)

	defer redisCloseClient()

	// create a new instance of the Deps struct
	// We pass this struct into the handlers so they can access the discord client
	// and kill signal
	dependencies := &deps.Deps{RedisClient: redisClient, Validate: validate}

	// Set up the router, connected to discord functionality
	router := chi.NewRouter()
	router.Use(auth.ApiKeyAuthMiddleware(Config.Environment.ApiKey))
	// Accepts join request and joins the voice channel
	router.Post("/join", calls.JoinVoiceChannel(dependencies))
	// Accepts leave request and leaves the voice channel
	router.Post("/{bot_id}/{guild_id}/leave", calls.LeaveVoiceChannel(dependencies))
	// Accepts a Config object and sets the responder config
	router.Post("/{bot_id}/{guild_id}/config", calls.UpdateConfig(dependencies))
	// Subscribes to the transcript SSE stream, which sends lines of the transcript as strings when new lines are available
	router.Get("/{bot_id}/{guild_id}/transcript", calls.TranscriptSSEHandler(dependencies))
	// Subscribes to the tool messages SSE stream, which sends tool messages as strings when the responder sends them
	router.Get("/{bot_id}/{guild_id}/tool-messages", calls.ToolMessagesSSEHandler(dependencies))

	// Start the REST API server
	log.Info("Starting REST API server on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("Error starting REST API server: %v", err)
	}
}
