package llm

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	openai "com.deablabs.teno-voice/internal/llm/openai"
	goOpenai "github.com/sashabaranov/go-openai"
)

func GetTranscriptResponseStream(transcript string, service string, model string, botName string, personality string, toolList []string, cache string) (*goOpenai.ChatCompletionStream, error) {
	pb := NewPromptBuilder(botName, transcript, personality, toolList, cache)

	pb.AddIntroduction()

	pb.AddTools()

	pb.AddCache()

	pb.AddTranscript()

	prompt := pb.Build()

	log.Printf("Prompt: %s", prompt)

	switch service {
	case "openai":
		return openai.CreateOpenAIStream(model, prompt, 1000)
	default:
		return nil, errors.New("service not found")
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
	cacheIntro := "Below is a list of cached items. Each cached item is represented by a unique identifier (ID) and has three properties: `Type`: indicates what the cache item represents (e.g., user message, tool response, task), `Permanent`: a boolean indicating whether the item should always remain in the cache, `Content`: the actual content of the cache item. You should always consider the cache when formulating your responses (especially pending tasks) and decide whether any of the cache items can be deleted or have been completed. If you decide to delete a cache item, use the `EditCache` tool below, as you'd use any other tool."
	cacheEditTool := "\n\nTool: EditCache\n`Description`: This tool allows you to delete an item from the cache when it is no longer needed or if the item represents a task that has been completed. To use this tool, provide the `ID` of the cache item you wish to delete.\n`Input Guide`: { \"name\": \"EditCache\", \"input\": \"<cache item ID>\" }\n`Output Guide`: The tool does not return a value, but successfully using it will remove the specified item from the cache."
	cacheContent := fmt.Sprintf("\n\nCache:\n%s", pb.cache)
	pb.sections = append(pb.sections, cacheIntro, cacheEditTool, cacheContent)
	return pb
}

// Build concatenates all sections and returns the final prompt
func (pb *PromptBuilder) Build() string {
	return strings.Join(pb.sections, "\n\n")
}
