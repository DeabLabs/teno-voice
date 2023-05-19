package openai

import (
	"context"
	"errors"

	Config "com.deablabs.teno-voice/internal/config"
	"com.deablabs.teno-voice/internal/llm/promptbuilder"
	"com.deablabs.teno-voice/internal/llm/tiktoken"
	"com.deablabs.teno-voice/internal/usage"
	goOpenai "github.com/sashabaranov/go-openai"
)

type OpenAIConfig struct {
	Model string `validate:"required"`
}

type OpenAILLM struct {
	Config OpenAIConfig
}

func NewOpenAILLM(config OpenAIConfig) *OpenAILLM {
	return &OpenAILLM{
		Config: config,
	}
}

var openAiToken = Config.Environment.OpenAIToken

func (o *OpenAILLM) GetTranscriptResponseStream(transcript string, botName string, promptContents *promptbuilder.PromptContents) (*goOpenai.ChatCompletionStream, usage.LLMEvent, error) {
	pb := promptbuilder.NewPromptBuilder(botName, transcript, promptContents.Personality, promptContents.ToolList, promptContents.Cache, promptContents.Tasks)

	pb.AddIntroduction()

	pb.AddTools()

	pb.AddCache()

	pb.AddTasks()

	pb.AddTranscript()

	prompt := pb.Build()

	stream, err := CreateOpenAIStream(o.Config.Model, prompt, 1000)
	if err != nil {
		return nil, usage.LLMEvent{}, err
	}

	usageEvent := usage.NewLLMEvent("service", o.Config.Model, tiktoken.TokenCount(prompt, o.Config.Model), 0)

	return stream, *usageEvent, err

}

func CreateOpenAIStream(model string, prompt string, maxTokens int) (*goOpenai.ChatCompletionStream, error) {
	// Initialize OpenAI client
	c := goOpenai.NewClient(openAiToken)
	ctx := context.Background()

	// log.Printf("Prompt: " + prompt)

	// Set up the request to OpenAI with the required parameters
	req := goOpenai.ChatCompletionRequest{
		Model:     model,
		MaxTokens: maxTokens,
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
		return nil, errors.New("ChatCompletionStream error: " + err.Error())
	}

	return stream, nil
}
