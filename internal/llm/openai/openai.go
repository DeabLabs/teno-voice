package openai

import (
	"context"
	"errors"

	"com.deablabs.teno-voice/internal/llm/promptbuilder"
	"com.deablabs.teno-voice/internal/llm/tiktoken"
	"com.deablabs.teno-voice/internal/transcript"
	"com.deablabs.teno-voice/internal/usage"
	goOpenai "github.com/sashabaranov/go-openai"
)

type OpenAIConfig struct {
	ApiKey string `validate:"required"`
	Model  string `validate:"required"`
}

type OpenAILLM struct {
	Config OpenAIConfig
	client *goOpenai.Client
}

func NewOpenAILLM(config OpenAIConfig) *OpenAILLM {
	return &OpenAILLM{
		Config: config,
		client: goOpenai.NewClient(config.ApiKey),
	}
}

func (o *OpenAILLM) GetTranscriptResponseStream(transcript *transcript.Transcript, botName string, promptContents *promptbuilder.PromptContents) (*goOpenai.ChatCompletionStream, usage.LLMEvent, error) {
	pb := promptbuilder.NewPromptBuilder(botName, transcript, promptContents)

	pb.AddBotPrimer()

	if promptContents.Tools != nil {
		pb.AddToolPrimer()
		pb.AddTools()
	}

	if promptContents.Documents != nil {
		pb.AddDocumentPrimer()
		pb.AddDocs()
	}

	if promptContents.Tasks != nil {
		pb.AddTaskPrimer()
		pb.AddTasks()
	}

	pb.AddTranscriptPrimer()

	systemContent := pb.Build()

	systemMessage := goOpenai.ChatCompletionMessage{
		Role:    "system",
		Content: systemContent,
	}

	messages := []goOpenai.ChatCompletionMessage{}

	messages = append(messages, systemMessage)

	transcriptMessages, transcriptString := transcript.ToChatCompletionMessages()

	// log.Print("Transcript: " + transcriptString)

	messages = append(messages, transcriptMessages...)

	c := o.client
	ctx := context.Background()

	req := goOpenai.ChatCompletionRequest{
		Model:     o.Config.Model,
		MaxTokens: 1000,
		Messages:  messages,
		Stream:    true,
	}

	// Log prompt
	// for _, message := range messages {
	// 	log.Print("[" + message.Role + "] " + message.Content)
	// }

	stream, err := c.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, usage.LLMEvent{}, errors.New("ChatCompletionStream error: " + err.Error())
	}

	usageEvent := usage.NewLLMEvent("service", o.Config.Model, tiktoken.TokenCount(systemContent+transcriptString, o.Config.Model), 0)

	return stream, *usageEvent, err
}
