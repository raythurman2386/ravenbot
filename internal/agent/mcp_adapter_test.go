package agent

import (
	"encoding/json"
	"testing"

	"github.com/raythurman2386/ravenbot/internal/mcp"
	"google.golang.org/genai"
)

func TestConvertMCPSchemaToGenAI(t *testing.T) {
	rawSchema := `
	{
		"type": "object",
		"properties": {
			"location": {
				"type": "string",
				"description": "The city and state, e.g. San Francisco, CA"
			},
			"unit": {
				"type": "string",
				"enum": ["celsius", "fahrenheit"]
			}
		},
		"required": ["location"]
	}`

	schema, err := convertMCPSchemaToGenAI(json.RawMessage(rawSchema))
	if err != nil {
		t.Fatalf("Failed to convert schema: %v", err)
	}

	if schema.Type != genai.TypeObject {
		t.Errorf("Expected TypeObject, got %v", schema.Type)
	}

	if len(schema.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(schema.Properties))
	}

	if schema.Properties["location"].Type != genai.TypeString {
		t.Errorf("Expected location to be String, got %v", schema.Properties["location"].Type)
	}

	if len(schema.Required) != 1 || schema.Required[0] != "location" {
		t.Errorf("Expected required field 'location', got %v", schema.Required)
	}
}

func TestMCPToolToGenAI(t *testing.T) {
	mcpTool := mcp.Tool{
		Name:        "get_weather",
		Description: "Get the current weather in a given location",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"location": {"type": "string"}
			}
		}`),
	}

	genTool, err := mcpToolToGenAI("weather-server", mcpTool)
	if err != nil {
		t.Fatalf("Failed to convert tool: %v", err)
	}

	expectedName := "weather-server_get_weather"
	if genTool.Name != expectedName {
		t.Errorf("Expected name %s, got %s", expectedName, genTool.Name)
	}

	if genTool.Description != "[weather-server] Get the current weather in a given location" {
		t.Errorf("Unexpected description: %s", genTool.Description)
	}
}
