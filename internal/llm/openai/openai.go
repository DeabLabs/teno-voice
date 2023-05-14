package openai

import (
	"context"
	"errors"

	Config "com.deablabs.teno-voice/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

var openAiToken = Config.Environment.OpenAIToken

func CreateOpenAIStream(model string, prompt string, maxTokens int) (*openai.ChatCompletionStream, error) {
	// Initialize OpenAI client
	c := openai.NewClient(openAiToken)
	ctx := context.Background()

	// log.Printf("Prompt: " + prompt)

	// Set up the request to OpenAI with the required parameters
	req := openai.ChatCompletionRequest{
		Model:     model,
		MaxTokens: maxTokens,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Stream: true,
	}

	// Create the chat completion stream
	stream, err := c.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, errors.New("ChatCompletionStream error: " + err.Error())
	}

	return stream, nil
}
