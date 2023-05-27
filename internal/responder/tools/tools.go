package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputGuide  string `json:"inputGuide"`
	OutputGuide string `json:"outputGuide"`
}

type ToolMessage struct {
	Name  string `json:"name"`
	Input string `json:"input"`
}

func FormatToolMessage(message string, availableTools []Tool) string {
	lastBracket := strings.LastIndex(message, "]")
	if lastBracket != -1 {
		message = message[:lastBracket+1]
	}

	var toolMessages []ToolMessage
	err := json.Unmarshal([]byte(message), &toolMessages)
	// If there's an error, the JSON was invalid. Return an empty list.
	if err != nil {
		// Log the specific unmarshalling error for debugging.
		fmt.Printf("Tool message JSON unmarshal error: %v\n", err)
		return ""
	}

	// Convert the list of available tools into a set for faster lookup.
	availableToolNames := make(map[string]bool)
	for _, tool := range availableTools {
		availableToolNames[tool.Name] = true
	}

	validToolMessages := make([]ToolMessage, 0)

	// Iterate through the tool messages, checking that each has a non-empty "name" and "input",
	// and that the "name" corresponds to an available tool. Keep only valid tool messages.
	for _, toolMessage := range toolMessages {
		// Trim leading and trailing whitespace from the name and input.
		name := strings.TrimSpace(toolMessage.Name)
		input := strings.TrimSpace(toolMessage.Input)

		if name == "" {
			fmt.Printf("Invalid tool message: name is empty\n")
		}
		if input == "" {
			fmt.Printf("Invalid tool message: input is empty\n")
		}
		if !availableToolNames[name] {
			fmt.Printf("Invalid tool message: tool name '%s' is not available\n", name)
		}

		if name != "" && input != "" && availableToolNames[name] {
			validToolMessages = append(validToolMessages, toolMessage)
		}
	}

	// Convert the valid tool messages back into a JSON string.
	validToolMessagesJSON, err := json.Marshal(validToolMessages)
	if err != nil {
		// Log the error and return an empty JSON array.
		fmt.Printf("Error marshalling tool messages back into JSON: %v\n", err)
		return ""
	}

	return string(validToolMessagesJSON)
}

// ParseTools parses a JSON string into an array of Tools
func ParseTools(jsonTools string) ([]Tool, error) {
	var tools []Tool
	err := json.Unmarshal([]byte(jsonTools), &tools)
	return tools, err
}

// ToolToString converts a Tool object to a string
func ToolToString(tool Tool) string {
	return fmt.Sprintf("Name: %s\nDescription: %s\nInput Guide: %s\nOutput Guide: %s", tool.Name, tool.Description, tool.InputGuide, tool.OutputGuide)
}

// ToolsToStringArray converts an array of Tool objects to an array of strings
func ToolsToStringArray(tools []Tool) []string {
	toolStrings := make([]string, len(tools))
	for i, tool := range tools {
		toolStrings[i] = ToolToString(tool)
	}
	return toolStrings
}
