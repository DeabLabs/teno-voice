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

func IsValidToolMessage(message string, availableTools []Tool) bool {
	lastBracket := strings.LastIndex(message, "]")
	if lastBracket != -1 {
		message = message[:lastBracket+1]
	}

	var toolMessages []ToolMessage
	err := json.Unmarshal([]byte(message), &toolMessages)

	// If there's an error, the JSON was invalid.
	if err != nil {
		// Log the specific unmarshalling error for debugging.
		fmt.Printf("Tool message JSON unmarshal error: %v\n", err)
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
		// Trim leading and trailing whitespace from the name and input.
		name := strings.TrimSpace(toolMessage.Name)
		input := strings.TrimSpace(toolMessage.Input)

		if name == "" || input == "" || !availableToolNames[name] {
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
