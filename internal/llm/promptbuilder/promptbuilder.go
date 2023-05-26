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
	Tools       []tools.Tool
	Documents   []Document
	Tasks       []Task
}

type PromptBuilder struct {
	botName     string
	transcript  string
	personality string
	tools       string
	documents   string
	tasks       string
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
func NewPromptBuilder(botName, transcript, personality string, tools []tools.Tool, documents []Document, tasks []Task) *PromptBuilder {
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

	var toolsString string
	if len(tools) > 0 {
		toolListJson, err := json.Marshal(tools)
		if err != nil {
			fmt.Printf("Error marshalling tool list: %s", err)
			toolsString = "[Error marshalling tool list]"
		} else {
			toolsString = string(toolListJson)
		}
	} else {
		toolsString = "[No additional tools available]"
	}

	var tasksString string
	if len(tasks) > 0 {
		taskListJson, err := json.Marshal(tasks)
		if err != nil {
			fmt.Printf("Error marshalling task list: %s", err)
			tasksString = "[Error marshalling task list]"
		} else {
			tasksString = string(taskListJson)
		}
	} else {
		tasksString = "[No pending tasks]"
	}

	return &PromptBuilder{
		botName:     botName,
		transcript:  transcript,
		personality: personality,
		tools:       toolsString,
		documents:   docString,
		tasks:       tasksString,
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
	toolPrimer := fmt.Sprintf("Below is a list of available tools you can use. These are your tools, and they aren't visible to anyone else in the voice channel. Each tool has four attributes: `Name`: the tool's identifier, `Description`: explains the tool's purpose and when to use it, `Input Guide`: advises on how to format the input string, `Output Guide`: describes the tool's return value, if any. To use a tool, you will append a tool message at the end of your normal spoken response, separated by a newline and a pipe ('|'). The spoken response is a string of text to be read aloud via TTS. You don't need to write a spoken response to use a tool, your response can simply be a | and then a tool command, in which case your tool command will be processed without any speech playing in the voice channel. Write all tool commands in the form of a JSON array. Each array element is a JSON object representing a tool command, with two properties: `name` and `input`. You shouldn't explain to the other voice call members how you use the tools unless someone asks. Here's an example of a response that uses a tool:\n\n[01:48:40] %s: This text before the pipe will be played to everyone in the voice channel like normal.\n|[{ \"name\": \"Tool1\", \"input\": \"This input will be sent to tool 1\" }, { \"name\": \"Tool2\", \"input\": \"This input will be sent to tool 2\" }]\n\nRemember to enter a new line and write a '|' before writing your tool message. Review the `description`, `input guide`, and `output guide` of each tool carefully to use them effectively.", pb.botName)

	tools := "Tool List:\n" + pb.tools + "\n"

	pb.sections = append(pb.sections, toolPrimer, tools)
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
	taskPrimer := "Below is a list of pending tasks. Each task is represented by its `Name`, `Description`, and `DeliverableGuide`. The `Description` details the task at hand, and the `DeliverableGuide` how to complete the task, whether its the use of a specific tool and/or relaying particular information to someone in the call. Here's an example of a task:\n\nName: Inform about weather\nDescription: Share the current weather conditions with the group using the Weather Tool.\nDeliverableGuide: Share the output of the Weather tool with the group. These are your tasks, but you may need to ask people in the call for information to complete them. Always take your pending tasks into account when responding, and make every effort to complete them. If the last line of the transcript is telling you to complete pending tasks, attempt to complete them, or mark them done using the associated tools if they are already complete. Do not talk about your tasks in the voice call unless people explicitly ask about them. If you are completing a task, you can simply write the tool message, you don't need to mention it in the voice channel."

	tasks := "Task List:\n" + pb.tasks + "\n"

	pb.sections = append(pb.sections, taskPrimer, tasks)
	return pb
}

// Build concatenates all sections and returns the final prompt
func (pb *PromptBuilder) Build() string {
	return strings.Join(pb.sections, "\n\n")
}
