package texttospeech

import (
	"encoding/json"
	"fmt"
	"io"

	"com.deablabs.teno-voice/internal/textToSpeech/azure"
	"github.com/go-playground/validator/v10"
)

type TTSConfigPayload struct {
	TTSServiceName string      `validate:"required"`
	TTSConfig      interface{} `validate:"required"`
}

type TextToSpeechService interface {
	Synthesize(text string) (io.ReadCloser, error)
}

func TTSConfigValidation(fl validator.FieldLevel) bool {
	config, ok := fl.Field().Interface().(TTSConfigPayload)
	if !ok {
		return false
	}

	// Check the service name
	if config.TTSServiceName == "" {
		return false
	}

	// Add more checks based on the service type
	switch config.TTSServiceName {
	case "azure":
		// Cast to Azure's config and validate
		azureConfig, ok := config.TTSConfig.(azure.AzureConfig)
		if !ok {
			return false
		}

		// Validate azureConfig
		validateAzure := validator.New()
		err := validateAzure.Struct(azureConfig)
		if err != nil {
			return false
		}
	default:
		// Unknown service
		return false
	}

	return true
}

func ParseTTSConfig(payload TTSConfigPayload) (TextToSpeechService, error) {
	switch payload.TTSServiceName {
	case "azure":
		var config azure.AzureConfig
		configData, err := json.Marshal(payload.TTSConfig)
		if err != nil {
			err := fmt.Errorf("error marshalling config: %s", err)
			return nil, err
		}
		err = json.Unmarshal(configData, &config)
		if err != nil {
			err := fmt.Errorf("error unmarshalling config: %s", err)
			return nil, err
		}

		return azure.NewAzureTTS(config), nil
	// case "elevenlabs":
	// 	var config ElevenLabsConfig
	// 	configData, err := json.Marshal(payload.TextToSpeechConfig)
	// 	if err != nil {
	// 		// handle error
	// 	}
	// 	err = json.Unmarshal(configData, &config)
	// 	if err != nil {
	// 		// handle error
	// 	}

	// 	return NewElevenLabsTTS(config), nil

	default:
		err := fmt.Errorf("unknown text to speech service: %s", payload.TTSServiceName)
		return nil, err
	}
}
