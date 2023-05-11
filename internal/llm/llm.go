package llm

import (
	"errors"
	"fmt"
	"log"
	"time"

	openai "com.deablabs.teno-voice/internal/llm/openai"
	goOpenai "github.com/sashabaranov/go-openai"
)

func GetTranscriptResponseStream(transcript string, service string, model string, botName string, personality string) (*goOpenai.ChatCompletionStream, error) {
	prompt := formatPrompt(botName, transcript, personality)
	log.Printf("Prompt: %s", prompt)

	switch service {
	case "openai":
		return openai.CreateOpenAIStream(model, prompt, 1000)
	default:
		return nil, errors.New("service not found")
	}
}

func formatPrompt(botName string, transcript string, personality string) string {
	prompt := "You are %[1]s, and you will participate in a discord voice channel. Here's a description of your personality, your responses should always be from this perspective:\n\n%[3]s\n\n. Your response will be played through a text-to-speech system in the voice call. Below is the transcript of a voice call, up to the current moment. It may include transcription errors, if you think a transcription was incorrect, infer the true words from context. The first sentence of your response should be as short as possible within reason. If the last person to speak doesn't expect or want a response from you, or they are explicitly asking you to stop speaking, your response should only be the single character '^' with no spaces.\n\n%[2]s\n[%[4]s] %[1]s:"
	return fmt.Sprintf(prompt, botName, transcript, personality, time.Now().Format("15:04:05"))
}
