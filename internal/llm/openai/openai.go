package openai

import (
	"context"
	"errors"

	openai "github.com/sashabaranov/go-openai"
)

func CreateOpenAIStream(model string, prompt string, maxTokens int) (*openai.ChatCompletionStream, error) {
	// Initialize OpenAI client
	c := openai.NewClient("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")
	ctx := context.Background()

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
