package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"com.deablabs.teno-voice/internal/deps"
	"com.deablabs.teno-voice/internal/discord"
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/log"
	"github.com/go-chi/chi"
	"github.com/joho/godotenv"
)

func main() {
	log.Info("starting up")

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	token := os.Getenv("TOKEN")

	s := make(chan os.Signal, 1)

	// create discord client
	client, err := disgo.New(token,
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuilds, gateway.IntentGuildVoiceStates, gateway.IntentGuildMessages)),
	)

	if err != nil {
		log.Fatal("error creating client: ", err)
	}

	// close the client when the program exits
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		client.Close(ctx)
	}()

	// open the gateway connection to discord
	if err = client.OpenGateway(context.TODO()); err != nil {
		log.Fatal("error connecting to gateway: ", err)
	}

	// wait for the signal for the client to be ready
	log.Info("Waiting for discord client to be ready")

	// create a new instance of the Deps struct
	// We pass this struct into the handlers so they can access the discord client
	// and kill signal
	dependencies := &deps.Deps{DiscordClient: &client, Signal: s}

	// Set up the router, connected to discord functionality
	router := chi.NewRouter()
	router.Post("/join", discord.JoinVoiceCall(dependencies))
	router.Post("/leave", discord.LeaveVoiceCall(dependencies))

	// Start the REST API server
	log.Info("Starting REST API server on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("Error starting REST API server: %v", err)
	}
}
