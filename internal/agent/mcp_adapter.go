package agent

import (
	"encoding/json"
	"fmt"

	"github.com/raythurman2386/ravenbot/internal/mcp"
	"google.golang.org/genai"
)

// convertMCPSchemaToGenAI converts a raw JSON Schema (from MCP) to genai.Schema
func convertMCPSchemaToGenAI(rawSchema json.RawMessage) (*genai.Schema, error) {
	var schemaMap map[string]any
	if err := json.Unmarshal(rawSchema, &schemaMap); err != nil {
		return nil, err
	}
	return buildGenAISchema(schemaMap), nil
}

func buildGenAISchema(schema map[string]any) *genai.Schema {
	t, _ := schema["type"].(string)

	// Map JSON Schema types to GenAI types
	var genType genai.Type
	switch t {
	case "object":
		genType = genai.TypeObject
	case "array":
		genType = genai.TypeArray
	case "string":
		genType = genai.TypeString
	case "number", "integer":
		genType = genai.TypeNumber // Gemini uses NUMBER for both usually, or INTEGER
		if t == "integer" {
			genType = genai.TypeInteger
		}
	case "boolean":
		genType = genai.TypeBoolean
	default:
		genType = genai.TypeString // Fallback
	}

	gs := &genai.Schema{
		Type: genType,
	}

	if desc, ok := schema["description"].(string); ok {
		gs.Description = desc
	}

	// Handle Object Properties
	if genType == genai.TypeObject {
		if props, ok := schema["properties"].(map[string]any); ok {
			gs.Properties = make(map[string]*genai.Schema)
			for k, v := range props {
				if vMap, ok := v.(map[string]any); ok {
					gs.Properties[k] = buildGenAISchema(vMap)
				}
			}
		}
		if req, ok := schema["required"].([]any); ok {
			for _, r := range req {
				if rStr, ok := r.(string); ok {
					gs.Required = append(gs.Required, rStr)
				}
			}
		}
	}

	// Handle Array Items
	if genType == genai.TypeArray {
		if items, ok := schema["items"].(map[string]any); ok {
			gs.Items = buildGenAISchema(items)
		}
	}

	return gs
}

// mcpToolToGenAI converts an mcp.Tool to a genai.FunctionDeclaration
func mcpToolToGenAI(serverName string, tool mcp.Tool) (*genai.FunctionDeclaration, error) {
	schema, err := convertMCPSchemaToGenAI(tool.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema for tool %s: %w", tool.Name, err)
	}

	// Namespace the tool name to avoid collisions
	namespacedName := fmt.Sprintf("%s_%s", serverName, tool.Name)

	return &genai.FunctionDeclaration{
		Name:        namespacedName,
		Description: fmt.Sprintf("[%s] %s", serverName, tool.Description),
		Parameters:  schema,
	}, nil
}

// mcpResourceToGenAI converts an mcp.Resource to a virtual tool declaration
func mcpResourceToGenAI(serverName string, res mcp.Resource) *genai.FunctionDeclaration {
	// Virtual tool name for reading this specific resource
	// We'll use a generic ReadResource tool instead of individual virtual tools for each URI
	// to keep the tool count manageable.
	return nil
}

// GetReadResourceTool returns a generic tool for reading MCP resources
func GetReadResourceTool() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "ReadMCPResource",
		Description: "Reads the content of an MCP resource from a specific server and URI.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"server": {
					Type:        genai.TypeString,
					Description: "The name of the MCP server.",
				},
				"uri": {
					Type:        genai.TypeString,
					Description: "The URI of the resource to read.",
				},
			},
			Required: []string{"server", "uri"},
		},
	}
}
