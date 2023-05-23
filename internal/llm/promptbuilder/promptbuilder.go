package promptbuilder

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"com.deablabs.teno-voice/internal/responder/tools"
)

type PromptContents struct {
	Personality string `validate:"required"`
	ToolList    []tools.Tool
	Documents   []Document
	Tasks       []Task
}

type PromptBuilder struct {
	botName     string
	transcript  string
	personality string
	toolList    string
	documents   string
	taskList    string
	sections    []string
}

type Document struct {
	Name    string `validate:"required"`
	Content string `validate:"required"`
}

type Task struct {
	Name             string `validate:"required"`
	Description      string `validate:"required"`
	DeliverableGuide string `validate:"required"`
}

// NewPromptBuilder creates a new PromptBuilder with default values
func NewPromptBuilder(botName, transcript, personality string, toolList []tools.Tool, documents []Document, taskList []Task) *PromptBuilder {
	var docString string

	if len(documents) > 0 {
		docJson, err := json.Marshal(documents)
		if err != nil {
			fmt.Printf("Error marshalling documents: %s", err)
			docString = "[Error marshalling documents]"
		} else {
			docString = string(docJson)
		}
	} else {
		docString = "[No documents]"
	}

	var toolListString string
	if len(toolList) > 0 {
		toolListJson, err := json.Marshal(toolList)
		if err != nil {
			fmt.Printf("Error marshalling tool list: %s", err)
			toolListString = "[Error marshalling tool list]"
		} else {
			toolListString = string(toolListJson)
		}
	} else {
		toolListString = "[No additional tools available]"
	}

	var taskListString string
	if len(taskList) > 0 {
		taskListJson, err := json.Marshal(taskList)
		if err != nil {
			fmt.Printf("Error marshalling task list: %s", err)
			taskListString = "[Error marshalling task list]"
		} else {
			taskListString = string(taskListJson)
		}
	} else {
		taskListString = "[No pending tasks]"
	}

	return &PromptBuilder{
		botName:     botName,
		transcript:  transcript,
		personality: personality,
		toolList:    toolListString,
		documents:   docString,
		taskList:    taskListString,
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

	toolList := "Tool List:\n" + pb.toolList + "\n"

	pb.sections = append(pb.sections, toolPrimer, toolList)
	return pb
}

// AddCache adds the cache section to the prompt
func (pb *PromptBuilder) AddDocs() *PromptBuilder {
	documents := fmt.Sprintf("\n\nDocuments:\n%s", pb.documents)
	pb.sections = append(pb.sections, documents)
	return pb
}

// AddTasks adds the tasks section to the prompt
func (pb *PromptBuilder) AddTasks() *PromptBuilder {
	taskPrimer := "Below is a list of pending tasks. Each task is represented by its `Name`, `Description`, and `DeliverableGuide`. The `Description` details the task at hand, and the `DeliverableGuide` provides guidance on what constitutes successful completion of the task, such as the use of a specific tool or relaying particular information to someone in the call. Your responses should always consider these tasks, and you should make every effort to complete them when appropriate. Here's an example:\n\nName: Inform about weather\nDescription: Share the current weather conditions with the group using the Weather Tool.\nDeliverableGuide: Share the output of the Weather tool with the group."

	taskList := "Task List:\n" + pb.taskList + "\n"

	pb.sections = append(pb.sections, taskPrimer, taskList)
	return pb
}

// Build concatenates all sections and returns the final prompt
func (pb *PromptBuilder) Build() string {
	return strings.Join(pb.sections, "\n\n")
}
