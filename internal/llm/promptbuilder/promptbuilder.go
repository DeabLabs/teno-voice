package promptbuilder

import (
	"encoding/json"
	"fmt"
	"strings"

	"com.deablabs.teno-voice/internal/responder/tools"
	"com.deablabs.teno-voice/internal/transcript"
)

type PromptContents struct {
	BotPrimer              string `validate:"required"`
	CustomTranscriptPrimer string
	CustomToolPrimer       string
	CustomDocumentPrimer   string
	CustomTaskPrimer       string
	Tools                  []tools.Tool
	Documents              []Document
	Tasks                  []Task
}

type PromptBuilder struct {
	botName        string
	promptContents *PromptContents
	sections       []string
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

var defaultTranscriptPrimer = "Below is the transcript of the voice channel, up to the current moment. It may include transcription errors or dropped words (especially at the beginnings of lines), if you think a transcription was incorrect, infer the true words from context. The first sentence of your response should be as short as possible within reason. The transcript may also include information like your previous tool uses, and mark when others interrupted you to stop your words from playing (which may mean they want you to stop talking). If the last person to speak doesn't expect or want a response from you, or they are explicitly asking you to stop speaking, your response should only be the single character '^' with no spaces."

var defaultToolPrimer = "Below is a list of available tools you can use. These are your tools, and they aren't visible to anyone else in the voice channel. Each tool has four attributes: `Name`: the tool's identifier, `Description`: explains the tool's purpose and when to use it, `Input Guide`: advises on how to format the input string, `Output Guide`: describes the tool's return value, if any. To use a tool, you will append a tool message at the end of your normal spoken response, separated by a pipe ('|'). The spoken response is a string of text to be read aloud via TTS. You don't need to write a spoken response to use a tool, your response can simply be a | and then a tool command, in which case your tool command will be processed without any speech playing in the voice channel. Write all tool commands in the form of a JSON array. Each array element is a JSON object representing a tool command, with two properties: `name` and `input`. You shouldn't explain to the other voice call members how you use the tools unless someone asks. Here's an example of a response that uses a tool:\n\nSure thing, I will send a message to the general channel. |[{ \"name\": \"SendMessageToGeneralChannel\", \"input\": \"Hello!\" }]\n\nRemember to write a '|' before writing your tool message. Review the `description`, `input guide`, and `output guide` of each tool carefully to use them effectively."

var defaultTaskPrimer = "Below is a list of pending tasks. Each task is represented by its `Name`, `Description`, and `DeliverableGuide`. The `Description` details the task at hand, and the `DeliverableGuide` how to complete the task, whether its the use of a specific tool and/or relaying particular information to someone in the call. These are your tasks, but you may need to ask people in the call for information to complete them. Always take your pending tasks into account when responding, and make every effort to complete them. If the last line of the transcript is telling you to complete pending tasks, attempt to complete them, or mark them done using the associated tools if they are already complete. Do not talk about your tasks in the voice call unless people explicitly ask about them. If you are completing a task, you can simply write the tool message, you don't need to mention it in the voice channel."

var defaultDocumentPrimer = "Below is a list of documents for you to reference when responding in the voice channel."

func NewPromptBuilder(botName string, transcript *transcript.Transcript, promptContents *PromptContents) *PromptBuilder {
	return &PromptBuilder{
		botName:        botName,
		promptContents: promptContents,
		sections:       make([]string, 0, 10),
	}
}

// AddBotPrimer adds the bot primer section to the prompt
func (pb *PromptBuilder) AddBotPrimer() *PromptBuilder {
	pb.sections = append(pb.sections, pb.promptContents.BotPrimer)
	return pb
}

// AddTranscriptPrimer adds the transcript primer section to the prompt
func (pb *PromptBuilder) AddTranscriptPrimer() *PromptBuilder {
	if pb.promptContents.CustomTranscriptPrimer == "" {
		pb.sections = append(pb.sections, defaultTranscriptPrimer)
	} else {
		pb.sections = append(pb.sections, pb.promptContents.CustomTranscriptPrimer)
	}

	silenceInstruction := "If you don't want to say anything, respond with the single character '^'."
	pb.sections = append(pb.sections, silenceInstruction)
	return pb
}

// AddToolPrimer adds the tool primer section to the prompt
func (pb *PromptBuilder) AddToolPrimer() *PromptBuilder {
	if pb.promptContents.CustomToolPrimer == "" {
		pb.sections = append(pb.sections, defaultToolPrimer)
	} else {
		pb.sections = append(pb.sections, pb.promptContents.CustomToolPrimer)
	}
	return pb
}

// AddDocumentPrimer adds the document primer section to the prompt
func (pb *PromptBuilder) AddDocumentPrimer() *PromptBuilder {
	if pb.promptContents.CustomDocumentPrimer == "" {
		pb.sections = append(pb.sections, defaultDocumentPrimer)
	} else {
		pb.sections = append(pb.sections, pb.promptContents.CustomDocumentPrimer)
	}
	return pb
}

// AddTaskPrimer adds the task primer section to the prompt
func (pb *PromptBuilder) AddTaskPrimer() *PromptBuilder {
	if pb.promptContents.CustomTaskPrimer == "" {
		pb.sections = append(pb.sections, defaultTaskPrimer)
	} else {
		pb.sections = append(pb.sections, pb.promptContents.CustomTaskPrimer)
	}
	return pb
}

// AddTools adds the tool primer and tool list sections to the prompt
func (pb *PromptBuilder) AddTools() *PromptBuilder {
	var toolsString string
	if len(pb.promptContents.Tools) == 0 {
		toolsString = "[No tools available]"
	} else {
		toolsJson, err := json.Marshal(pb.promptContents.Tools)

		if err != nil {
			fmt.Printf("Error marshalling tools: %s", err)
			toolsString = "[No tools available]"
		} else {
			toolsString = string(toolsJson)
		}
	}

	tools := "Tools:\n" + toolsString + "\n"

	pb.sections = append(pb.sections, tools)
	return pb
}

// AddDocs adds the documents section to the prompt
func (pb *PromptBuilder) AddDocs() *PromptBuilder {
	var docString string
	if len(pb.promptContents.Documents) == 0 {
		docString = "[No documents available]"
	} else {
		docJson, err := json.Marshal(pb.promptContents.Documents)
		if err != nil {
			fmt.Printf("Error marshalling documents: %s", err)
			docString = "[No documents available]"
		} else {
			docString = string(docJson)
		}
	}
	documents := fmt.Sprintf("\n\nDocuments:\n%s", docString)
	pb.sections = append(pb.sections, documents)
	return pb
}

// AddTasks adds the tasks section to the prompt
func (pb *PromptBuilder) AddTasks() *PromptBuilder {
	var tasksString string
	if len(pb.promptContents.Tasks) == 0 {
		tasksString = "[No pending tasks]"
	} else {
		tasksJson, err := json.Marshal(pb.promptContents.Tasks)
		if err != nil {
			fmt.Printf("Error marshalling task list: %s", err)
			tasksString = "[No pending tasks]"
		} else {
			tasksString = string(tasksJson)
		}
	}

	tasks := "Tasks:\n" + tasksString + "\n"

	pb.sections = append(pb.sections, tasks)
	return pb
}

// Build concatenates all sections and returns the final prompt
func (pb *PromptBuilder) Build() string {
	return strings.Join(pb.sections, "\n\n")
}
