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
//   - schema_valid:<path>           - Validates response against JSON schema
//   - tool_called:<name>            - Verifies specific tool was called
//   - no_tool_called                - Verifies no tools were called
//   - contains:<text>               - Checks if response contains text (case-insensitive)
//   - not_contains:<text>           - Checks if response doesn't contain text (case-insensitive)
//   - exact:<text>                  - Checks if response exactly matches text (case-sensitive)
//   - contains_any:[text1, text2]   - Checks if response contains any of the texts
//   - tool_args_contains:<json>     - Checks if tool arguments contain specific values
func RunCheck(check string, tr *trace.LLMTrace) CheckResult {
	// Handle YAML map format (e.g., contains: "text")
	// First try to parse as "type: value" format
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
		return validateSchema(tr, checkParam)

	case "tool_called":
		return checkToolCalled(tr, checkParam)

	case "no_tool_called":
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

	case "contains":
		return checkContains(tr, checkParam)

	case "not_contains":
		return checkNotContains(tr, checkParam)

	case "exact":
		return checkExact(tr, checkParam)

	case "contains_any":
		return checkContainsAny(tr, checkParam)

	case "tool_args_contains":
		return checkToolArgsContains(tr, checkParam)

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

// checkContains verifies that the response contains specific text.
func checkContains(tr *trace.LLMTrace, text string) CheckResult {
	result := CheckResult{
		Check:  "contains: " + text,
		Passed: false,
	}

	// Extract response text from the trace
	responseText := extractResponseText(tr)

	if strings.Contains(strings.ToLower(responseText), strings.ToLower(text)) {
		result.Passed = true
		result.Message = fmt.Sprintf("Response contains '%s'", text)
	} else {
		result.Message = fmt.Sprintf("Response does not contain '%s'", text)
	}

	return result
}

// checkNotContains verifies that the response does NOT contain specific text.
func checkNotContains(tr *trace.LLMTrace, text string) CheckResult {
	result := CheckResult{
		Check:  "not_contains: " + text,
		Passed: false,
	}

	responseText := extractResponseText(tr)

	if !strings.Contains(strings.ToLower(responseText), strings.ToLower(text)) {
		result.Passed = true
		result.Message = fmt.Sprintf("Response does not contain '%s'", text)
	} else {
		result.Message = fmt.Sprintf("Response contains '%s' (should not)", text)
	}

	return result
}

// checkExact verifies that the response exactly matches the specified text (case-sensitive).
func checkExact(tr *trace.LLMTrace, text string) CheckResult {
	result := CheckResult{
		Check:  "exact: " + text,
		Passed: false,
	}

	responseText := extractResponseText(tr)
	responseText = strings.TrimSpace(responseText)
	text = strings.TrimSpace(text)

	if responseText == text {
		result.Passed = true
		result.Message = "Response exactly matches expected text"
	} else {
		result.Message = fmt.Sprintf("Response does not match. Expected: '%s', Got: '%s'", text, responseText)
	}

	return result
}

// checkContainsAny verifies that the response contains at least one of the specified texts.
func checkContainsAny(tr *trace.LLMTrace, textsParam string) CheckResult {
	result := CheckResult{
		Check:  "contains_any: " + textsParam,
		Passed: false,
	}

	responseText := extractResponseText(tr)
	responseLower := strings.ToLower(responseText)

	// Parse the texts list (format: [text1, text2, text3] or "text1, text2, text3")
	texts := parseTextList(textsParam)

	foundTexts := []string{}
	for _, text := range texts {
		if strings.Contains(responseLower, strings.ToLower(text)) {
			foundTexts = append(foundTexts, text)
		}
	}

	if len(foundTexts) > 0 {
		result.Passed = true
		result.Message = fmt.Sprintf("Response contains: %s", strings.Join(foundTexts, ", "))
	} else {
		result.Message = fmt.Sprintf("Response does not contain any of: %s", strings.Join(texts, ", "))
	}

	return result
}

// checkToolArgsContains verifies that tool arguments contain specific key-value pairs.
func checkToolArgsContains(tr *trace.LLMTrace, argsParam string) CheckResult {
	result := CheckResult{
		Check:  "tool_args_contains",
		Passed: false,
	}

	if len(tr.ToolCalls) == 0 {
		result.Message = "No tools were called"
		return result
	}

	// Parse expected arguments as JSON or key:value pairs
	expectedArgs := parseToolArgs(argsParam)

	// Check each tool call's arguments
	for _, toolCall := range tr.ToolCalls {
		var actualArgs map[string]interface{}
		if err := json.Unmarshal(toolCall.Args, &actualArgs); err != nil {
			continue
		}

		// Check if all expected key-value pairs exist
		allMatch := true
		matchedPairs := []string{}
		for key, expectedValue := range expectedArgs {
			if actualValue, exists := actualArgs[key]; exists {
				// Compare values (convert to strings for simplicity)
				if strings.EqualFold(fmt.Sprintf("%v", actualValue), fmt.Sprintf("%v", expectedValue)) {
					matchedPairs = append(matchedPairs, fmt.Sprintf("%s=%v", key, expectedValue))
				} else {
					allMatch = false
					break
				}
			} else {
				allMatch = false
				break
			}
		}

		if allMatch && len(matchedPairs) > 0 {
			result.Passed = true
			result.Message = fmt.Sprintf("Tool '%s' arguments contain: %s", toolCall.Name, strings.Join(matchedPairs, ", "))
			return result
		}
	}

	result.Message = "Tool arguments do not match expected values"
	return result
}

// extractResponseText extracts the text content from a trace response.
func extractResponseText(tr *trace.LLMTrace) string {
	var responseData map[string]interface{}
	if err := json.Unmarshal(tr.Response.Body, &responseData); err != nil {
		// If not JSON, return raw body as string
		return string(tr.Response.Body)
	}

	// Try to extract text from common response formats
	// OpenAI format: choices[0].message.content
	if choices, ok := responseData["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok {
					return content
				}
			}
		}
	}

	// Anthropic format: content[].text
	if content, ok := responseData["content"].([]interface{}); ok {
		texts := []string{}
		for _, c := range content {
			if cMap, ok := c.(map[string]interface{}); ok {
				if text, ok := cMap["text"].(string); ok {
					texts = append(texts, text)
				}
			}
		}
		if len(texts) > 0 {
			return strings.Join(texts, " ")
		}
	}

	// Ollama/Custom format: message.content
	if message, ok := responseData["message"].(map[string]interface{}); ok {
		if content, ok := message["content"].(string); ok {
			return content
		}
	}

	// Fallback: return JSON as string
	return string(tr.Response.Body)
}

// parseTextList parses a list of texts from various formats.
func parseTextList(input string) []string {
	input = strings.TrimSpace(input)

	// Remove brackets if present: [text1, text2] -> text1, text2
	input = strings.TrimPrefix(input, "[")
	input = strings.TrimSuffix(input, "]")

	// Split by comma
	parts := strings.Split(input, ",")
	texts := []string{}
	for _, part := range parts {
		// Trim whitespace and quotes
		text := strings.TrimSpace(part)
		text = strings.Trim(text, "\"'")
		if text != "" {
			texts = append(texts, text)
		}
	}

	return texts
}

// parseToolArgs parses tool arguments from YAML/JSON format.
func parseToolArgs(input string) map[string]interface{} {
	args := make(map[string]interface{})

	// Try parsing as JSON first
	if err := json.Unmarshal([]byte(input), &args); err == nil {
		return args
	}

	// Fallback: parse as key:value pairs separated by newlines or commas
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse "key: value" format
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			// Remove quotes if present
			value = strings.Trim(value, "\"'")
			args[key] = value
		}
	}

	return args
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
