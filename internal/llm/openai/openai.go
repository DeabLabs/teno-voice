package openai

import (
	"context"
	"errors"

	"com.deablabs.teno-voice/internal/llm/promptbuilder"
	"com.deablabs.teno-voice/internal/llm/tiktoken"
	"com.deablabs.teno-voice/internal/usage"
	goOpenai "github.com/sashabaranov/go-openai"
)

type OpenAIConfig struct {
	ApiKey string `validate:"required"`
	Model  string `validate:"required"`
}

type OpenAILLM struct {
	Config OpenAIConfig
}

func NewOpenAILLM(config OpenAIConfig) *OpenAILLM {
	return &OpenAILLM{
		Config: config,
	}
}

func (o *OpenAILLM) GetTranscriptResponseStream(transcript string, botName string, promptContents *promptbuilder.PromptContents) (*goOpenai.ChatCompletionStream, usage.LLMEvent, error) {
	pb := promptbuilder.NewPromptBuilder(botName, transcript, promptContents.Personality, promptContents.Tools, promptContents.Documents, promptContents.Tasks)

	pb.AddIntroduction()

	pb.AddTools()

	pb.AddDocs()

	pb.AddTasks()

	pb.AddTranscript()

	prompt := pb.Build()

	c := goOpenai.NewClient(o.Config.ApiKey)
	ctx := context.Background()

	// log.Printf("Prompt: " + prompt)

	// Set up the request to OpenAI with the required parameters
	req := goOpenai.ChatCompletionRequest{
		Model:     o.Config.Model,
		MaxTokens: 1000,
		Messages: []goOpenai.ChatCompletionMessage{
			{
				Role:    goOpenai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Stream: true,
	}

	// Create the chat completion stream
	stream, err := c.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, usage.LLMEvent{}, errors.New("ChatCompletionStream error: " + err.Error())
	}

	usageEvent := usage.NewLLMEvent("service", o.Config.Model, tiktoken.TokenCount(prompt, o.Config.Model), 0)

	return stream, *usageEvent, err
}
