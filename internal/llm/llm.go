package llm

import (
	"errors"
	"fmt"
	"strings"
	"time"

	openai "com.deablabs.teno-voice/internal/llm/openai"
	"com.deablabs.teno-voice/internal/llm/tiktoken"
	"com.deablabs.teno-voice/internal/usage"
	goOpenai "github.com/sashabaranov/go-openai"
)

func GetTranscriptResponseStream(transcript string, service string, model string, botName string, personality string, toolList []string, cache string) (*goOpenai.ChatCompletionStream, *usage.LLMEvent, error) {
	pb := NewPromptBuilder(botName, transcript, personality, toolList, cache)

	pb.AddIntroduction()

	pb.AddTools()

	pb.AddCache()

	pb.AddTranscript()

	prompt := pb.Build()

	// log.Printf("Prompt: %s", prompt)

	usageEvent := usage.NewLLMEvent(service, model, tiktoken.TokenCount(prompt, model), 0)

	switch service {
	case "openai":
		stream, err := openai.CreateOpenAIStream(model, prompt, 1000)
		return stream, usageEvent, err
	default:
		return nil, &usage.LLMEvent{}, errors.New("service not found")
	}
}

type PromptBuilder struct {
	botName     string
	transcript  string
	personality string
	toolList    []string
	cache       string
	sections    []string
}

// NewPromptBuilder creates a new PromptBuilder with default values
func NewPromptBuilder(botName, transcript, personality string, toolList []string, cache string) *PromptBuilder {
	return &PromptBuilder{
		botName:     botName,
		transcript:  transcript,
		personality: personality,
		toolList:    toolList,
		cache:       cache,
	}
}

// AddIntroduction adds the introduction section to the prompt
func (pb *PromptBuilder) AddIntroduction() *PromptBuilder {
	introduction := fmt.Sprintf("Your name is %s. You will participate in a discord voice channel. Here's a description of your personality, your responses should always be from this perspective:\n\n%s", pb.botName, pb.personality)
	pb.sections = append(pb.sections, introduction)
	return pb
}

// AddTranscript adds the transcript section to the prompt
func (pb *PromptBuilder) AddTranscript() *PromptBuilder {
	transcript := fmt.Sprintf("Below is the transcript of a voice call, up to the current moment. It may include transcription errors (especially at the beginnings of lines), if you think a transcription was incorrect, infer the true words from context. The first sentence of your response should be as short as possible within reason. The transcript may also include information like your previous tool uses, and mark when others interrupted you to stop your words from playing (which may mean they want you to stop talking). If the last person to speak doesn't expect or want a response from you, or they are explicitly asking you to stop speaking, your response should only be the single character '^' with no spaces.\n\n%s\n[%s] %s:", pb.transcript, time.Now().Format("15:04:05"), pb.botName)
	pb.sections = append(pb.sections, transcript)
	return pb
}

// AddTools adds the tool primer and tool list sections to the prompt
func (pb *PromptBuilder) AddTools() *PromptBuilder {
	toolPrimer := fmt.Sprintf("Below is a list of available tools you can use. Each tool has four attributes: `Name`: the tool's identifier, `Description`: explains the tool's purpose and when to use it, `Input Guide`: advises on how to format the input string, `Output Guide`: describes the tool's return value, if any. To use a tool, compose a response with two parts: a spoken response and tool usage instructions, separated by a newline and a pipe ('|'). The spoken response is a string of text to be read aloud via TTS. The tool usage instructions are on the next line, starting with a '|', in the form of a JSON array. Each array element is a JSON object representing a tool to be used, with two properties: `name` and `input`. You shouldn't explain to the other voice call members how you use the tools unless someone asks. Here's an example of a response that uses a tool:\n\n[01:48:40] %s: This text before the pipe will be played in the voice channel like normal.\n|[{ \"name\": \"Tool1\", \"input\": \"This input will be sent to tool 1\" }, { \"name\": \"Tool2\", \"input\": \"This input will be sent to tool 2\" }].\n\nRemember to enter a new line and write a '|' before writing your tool message. Review the `description`, `input guide`, and `output guide` of each tool carefully to use them effectively.", pb.botName)

	// If there are no tools, say that instead
	toolList := ""
	if len(pb.toolList) == 0 {
		toolList = "Tool List:\nNo additional tools."
	} else {
		toolList = "Tool List:\n" + strings.Join(pb.toolList, "\n")
	}

	pb.sections = append(pb.sections, toolPrimer, toolList)
	return pb
}

// AddCache adds the cache section to the prompt
func (pb *PromptBuilder) AddCache() *PromptBuilder {
	cacheIntro := "Below is a list of cached items. Each cached item is represented by a unique identifier (ID) and has two properties: `Name`: The name of the item, which may indicate if it is a response from a tool, a task, or a piece of context to consider. `Content`: the actual content of the cache item. You should always consider the information and pending items in the cache when formulating your responses."
	cacheContent := fmt.Sprintf("\n\nCache:\n%s", pb.cache)
	pb.sections = append(pb.sections, cacheIntro, cacheContent)
	return pb
}

// Build concatenates all sections and returns the final prompt
func (pb *PromptBuilder) Build() string {
	return strings.Join(pb.sections, "\n\n")
}
