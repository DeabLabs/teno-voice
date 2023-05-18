package tools

import (
	"encoding/json"
	"fmt"
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

func IsValidToolMessage(message string, availableTools []Tool) bool {
	var toolMessages []ToolMessage
	err := json.Unmarshal([]byte(message), &toolMessages)

	// If there's an error, the JSON was invalid.
	if err != nil {
		return false
	}

	// Convert the list of available tools into a set for faster lookup.
	availableToolNames := make(map[string]bool)
	for _, tool := range availableTools {
		availableToolNames[tool.Name] = true
	}

	if len(toolMessages) == 0 {
		return false
	}

	// Iterate through the tool messages, checking that each has a non-empty "name" and "input",
	// and that the "name" corresponds to an available tool.
	for _, toolMessage := range toolMessages {
		if toolMessage.Name == "" || toolMessage.Input == "" || !availableToolNames[toolMessage.Name] {
			return false
		}
	}

	// If no invalid tool messages were found, the JSON is valid.
	return true
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
