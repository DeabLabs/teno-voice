package config

import (
	"log"

	env "github.com/Netflix/go-env"
	"github.com/joho/godotenv"
)

type Config struct {
	// DiscordToken is the token used to authenticate with Discord
	DiscordToken string `env:"DISCORD_TOKEN,required=true"`
	// AzureToken is the token used to authenticate with Azure
	AzureToken string `env:"AZURE_TOKEN,required=true"`
	// OpenAIToken is the token used to authenticate with OpenAI
	OpenAIToken string `env:"OPENAI_TOKEN,required=true"`
	// DeepgramToken is the token used to authenticate with Deepgram
	DeepgramToken string `env:"DEEPGRAM_TOKEN,required=true"`
}

var Environment = New()

func New() *Config {
	var environment Config

	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, parsing from environment")
	}

	es, err := env.UnmarshalFromEnviron(&environment)
	if err != nil {
		log.Fatal(err)
	}

	environment = Config{}
	err = env.Unmarshal(es, &environment)
	if err != nil {
		log.Fatal(err)
	}

	return &environment
}
