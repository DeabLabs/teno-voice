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
