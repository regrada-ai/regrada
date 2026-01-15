package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/matias/regrada/trace"
)

// RunCheck executes a single check against a trace.
// Supported checks:
//   - schema_valid:<path>     - Validates response against JSON schema
//   - tool_called:<name>      - Verifies specific tool was called
//   - no_tool_called          - Verifies no tools were called
func RunCheck(check string, tr *trace.LLMTrace) CheckResult {
	// Parse check type and parameters
	checkType := check
	var checkParam string

	if idx := strings.Index(check, ":"); idx > 0 {
		checkType = strings.TrimSpace(check[:idx])
		checkParam = strings.TrimSpace(check[idx+1:])
	}

	result := CheckResult{
		Check:  check,
		Passed: true,
	}

	// Run checks against actual trace data
	switch checkType {
	case "schema_valid":
		// Validate response against expected JSON schema
		return validateSchema(tr, checkParam)

	case "tool_called":
		// Verify specific tool was called
		return checkToolCalled(tr, checkParam)

	case "no_tool_called":
		// Verify no tools were called
		if len(tr.ToolCalls) == 0 {
			result.Passed = true
			result.Message = "No tools were called"
		} else {
			result.Passed = false
			toolNames := make([]string, len(tr.ToolCalls))
			for i, tc := range tr.ToolCalls {
				toolNames[i] = tc.Name
			}
			result.Message = fmt.Sprintf("Expected no tool calls, but got: %s", strings.Join(toolNames, ", "))
		}
		return result

	default:
		// Unknown check type
		result.Passed = false
		result.Message = fmt.Sprintf("Unknown check type: %s", checkType)
		return result
	}
}

// validateSchema validates the trace response against a JSON schema file.
func validateSchema(tr *trace.LLMTrace, schemaPath string) CheckResult {
	result := CheckResult{
		Check:  "schema_valid: " + schemaPath,
		Passed: false,
	}

	// Load the schema file
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		result.Message = fmt.Sprintf("Failed to load schema file: %v", err)
		return result
	}

	// Parse the schema
	var schema map[string]interface{}
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		result.Message = fmt.Sprintf("Failed to parse schema: %v", err)
		return result
	}

	// Parse the response body to get the actual output
	var responseData map[string]interface{}
	if err := json.Unmarshal(tr.Response.Body, &responseData); err != nil {
		result.Message = fmt.Sprintf("Failed to parse response body: %v", err)
		return result
	}

	// Basic schema validation
	// Check if required fields exist
	if requiredFields, ok := schema["required"].([]interface{}); ok {
		for _, field := range requiredFields {
			fieldName := field.(string)
			if _, exists := responseData[fieldName]; !exists {
				result.Message = fmt.Sprintf("Required field '%s' missing from response", fieldName)
				return result
			}
		}
	}

	// Validate properties types if specified
	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		for propName, propSchema := range properties {
			propSchemaMap := propSchema.(map[string]interface{})
			expectedType, hasType := propSchemaMap["type"].(string)

			if hasType {
				if actualValue, exists := responseData[propName]; exists {
					actualType := getJSONType(actualValue)
					if actualType != expectedType {
						result.Message = fmt.Sprintf("Field '%s' has type '%s', expected '%s'", propName, actualType, expectedType)
						return result
					}
				}
			}
		}
	}

	result.Passed = true
	result.Message = "Response matches expected schema"
	return result
}

// checkToolCalled verifies that a specific tool was called in the trace.
func checkToolCalled(tr *trace.LLMTrace, expectedToolName string) CheckResult {
	result := CheckResult{
		Check:  "tool_called: " + expectedToolName,
		Passed: false,
	}

	// Check if the expected tool was called
	for _, toolCall := range tr.ToolCalls {
		if toolCall.Name == expectedToolName {
			result.Passed = true
			result.Message = fmt.Sprintf("Tool '%s' was called", expectedToolName)
			return result
		}
	}

	// Tool was not called
	if len(tr.ToolCalls) == 0 {
		result.Message = fmt.Sprintf("Tool '%s' was not called (no tools were called)", expectedToolName)
	} else {
		toolNames := make([]string, len(tr.ToolCalls))
		for i, tc := range tr.ToolCalls {
			toolNames[i] = tc.Name
		}
		result.Message = fmt.Sprintf("Tool '%s' was not called (called: %s)", expectedToolName, strings.Join(toolNames, ", "))
	}

	return result
}

// getJSONType returns the JSON type name for a Go value.
func getJSONType(value interface{}) string {
	switch value.(type) {
	case string:
		return "string"
	case float64, int, int64:
		return "number"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}
