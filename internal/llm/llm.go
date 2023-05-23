package llm

import (
	"encoding/json"
	"fmt"

	"com.deablabs.teno-voice/internal/llm/openai"
	"com.deablabs.teno-voice/internal/llm/promptbuilder"
	"com.deablabs.teno-voice/internal/usage"
	"github.com/go-playground/validator/v10"
	goOpenai "github.com/sashabaranov/go-openai"
)

type LLMConfigPayload struct {
	LLMServiceName string      `validate:"required"`
	LLMConfig      interface{} `validate:"required"`
}

type LLMService interface {
	GetTranscriptResponseStream(transcript string, botName string, promptContents *promptbuilder.PromptContents) (*goOpenai.ChatCompletionStream, usage.LLMEvent, error)
}

func LLMConfigValidation(fl validator.FieldLevel) bool {
	config, ok := fl.Field().Interface().(LLMConfigPayload)
	if !ok {
		return false
	}

	// Check the service name
	if config.LLMServiceName == "" {
		return false
	}

	// Add more checks based on the service type
	switch config.LLMServiceName {
	case "openai":
		// Cast to OpenAI's config and validate
		openAIConfig, ok := config.LLMConfig.(openai.OpenAIConfig)
		if !ok {
			return false
		}

		// Validate openAIConfig
		validateOpenAI := validator.New()
		err := validateOpenAI.Struct(openAIConfig)
		if err != nil {
			return false
		}
	default:
		// Unknown service
		return false
	}

	return true
}

func ParseLLMConfig(payload LLMConfigPayload) (LLMService, error) {
	switch payload.LLMServiceName {
	case "openai":
		var config openai.OpenAIConfig
		configData, err := json.Marshal(payload.LLMConfig)
		if err != nil {
			err := fmt.Errorf("error marshalling config: %s", err)
			return nil, err
		}
		err = json.Unmarshal(configData, &config)
		if err != nil {
			err := fmt.Errorf("error unmarshalling config: %s", err)
			return nil, err
		}

		return openai.NewOpenAILLM(config), nil
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
		err := fmt.Errorf("unknown LLM service: %s", payload.LLMServiceName)
		return nil, err
	}
}
