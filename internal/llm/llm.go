package llm

import (
	"errors"

	openai "com.deablabs.teno-voice/internal/llm/openai"
	goOpenai "github.com/sashabaranov/go-openai"
)

func GetTranscriptResponseStream(transcript string, service string, model string) (*goOpenai.ChatCompletionStream, error) {
	prompt := "You are a friendly, interesting, and knowledgable conversation bot. Your response will be played through a text-to-speech system in the voice call. The following is the transcript of a voice call. It may include transcription errors, if you think a transcription was incorrect, infer the true words from context. Your response should be concise and to the point unless a user says otherwise. The first sentence of your response should be as short as possible within reason." + "\n\n" + transcript + "\n\n" + "Respond with your contribution to the conversation:" + "\n\n" + "[X:XX]Bot:"

	switch service {
	case "openai":
		return openai.CreateOpenAIStream(model, prompt, 1000)
	default:
		return nil, errors.New("service not found")
	}
}
