package cases

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

func LoadCase(path string) (Case, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Case{}, err
	}

	var c Case
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&c); err != nil {
		return Case{}, fmt.Errorf("parse case %s: %w", path, err)
	}

	if err := ValidateCase(c); err != nil {
		return Case{}, fmt.Errorf("validate case %s: %w", path, err)
	}

	c.Tags = sortedStrings(c.Tags)
	return c, nil
}

func ValidateCase(c Case) error {
	if c.ID == "" {
		return fmt.Errorf("id is required")
	}
	if len(c.Request.Messages) > 0 && len(c.Request.Input) > 0 {
		return fmt.Errorf("request must specify messages or input, not both")
	}
	if len(c.Request.Messages) == 0 && len(c.Request.Input) == 0 {
		return fmt.Errorf("request.messages or request.input is required")
	}
	for i, msg := range c.Request.Messages {
		if msg.Role == "" {
			return fmt.Errorf("request.messages[%d].role is required", i)
		}
		if msg.Content == "" {
			return fmt.Errorf("request.messages[%d].content is required", i)
		}
		if !isValidRole(msg.Role) {
			return fmt.Errorf("request.messages[%d].role must be system, user, assistant, or tool", i)
		}
	}
	if c.Assert != nil && c.Assert.JSON != nil {
		for path, exp := range c.Assert.JSON.Path {
			if path == "" {
				return fmt.Errorf("assert.json.path has empty key")
			}
			if err := exp.Validate(); err != nil {
				return fmt.Errorf("assert.json.path[%s]: %w", path, err)
			}
		}
	}
	return nil
}

func isValidRole(role string) bool {
	switch role {
	case "system", "user", "assistant", "tool":
		return true
	default:
		return false
	}
}

func WriteCase(path string, c Case) error {
	data, err := MarshalCase(c)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func MarshalCase(c Case) ([]byte, error) {
	c.Tags = sortedStrings(c.Tags)
	buf := &bytes.Buffer{}
	enc := yaml.NewEncoder(buf)
	enc.SetIndent(2)
	if err := enc.Encode(c); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func sortedStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}
