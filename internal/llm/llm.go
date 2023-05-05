package llm

import (
	"errors"

	openai "com.deablabs.teno-voice/internal/llm/openai"
	goOpenai "github.com/sashabaranov/go-openai"
)

func GetTranscriptResponseStream(transcript string, service string, model string) (*goOpenai.ChatCompletionStream, error) {
    prompt := "The following is the transcript of a voice call. You are a friendly conversation bot. Your response will be played through a text-to-speech system in the voice call. \n\n" + transcript + "\n\n" + "Respond with your contribution to the conversation:"

	switch service {
	case "openai":
		return openai.CreateOpenAIStream(model, prompt, 1000)
	default:
		return nil, errors.New("service not found")
	}
}
