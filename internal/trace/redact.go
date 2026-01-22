package trace

import (
	"regexp"
	"strings"

	"github.com/matias/regrada/internal/config"
)

type Redactor interface {
	Apply(t *Trace) []string
}

type RegexRedactor struct {
	patterns []compiledPattern
}

type compiledPattern struct {
	name        string
	regex       *regexp.Regexp
	replaceWith string
}

func NewRedactor(cfg config.RedactConfig) (*RegexRedactor, error) {
	var patterns []compiledPattern
	for _, pattern := range cfg.Patterns {
		re, err := regexp.Compile(pattern.Regex)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, compiledPattern{
			name:        pattern.Name,
			regex:       re,
			replaceWith: pattern.ReplaceWith,
		})
	}
	return &RegexRedactor{patterns: patterns}, nil
}

func (r *RegexRedactor) Apply(t *Trace) []string {
	if r == nil {
		return nil
	}
	var applied []string
	for i, msg := range t.Request.Messages {
		content, matched := applyPatterns(msg.Content, r.patterns)
		if len(matched) > 0 {
			t.Request.Messages[i].Content = content
			applied = append(applied, matched...)
		}
	}
	if t.Response.AssistantText != "" {
		content, matched := applyPatterns(t.Response.AssistantText, r.patterns)
		if len(matched) > 0 {
			t.Response.AssistantText = content
			applied = append(applied, matched...)
		}
	}
	return unique(applied)
}

func applyPatterns(input string, patterns []compiledPattern) (string, []string) {
	if input == "" {
		return input, nil
	}
	var matched []string
	output := input
	for _, pattern := range patterns {
		if pattern.regex.MatchString(output) {
			output = pattern.regex.ReplaceAllString(output, pattern.replaceWith)
			matched = append(matched, pattern.name)
		}
	}
	return output, matched
}

func unique(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func PresetPatterns(presets []string) []config.RedactPattern {
	var patterns []config.RedactPattern
	for _, preset := range presets {
		switch strings.ToLower(preset) {
		case "pii_basic":
			patterns = append(patterns,
				config.RedactPattern{Name: "email", Regex: `[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`, ReplaceWith: "[REDACTED_EMAIL]"},
				config.RedactPattern{Name: "phone", Regex: `\+?\d[\d\s().-]{7,}\d`, ReplaceWith: "[REDACTED_PHONE]"},
			)
		case "pii_strict":
			patterns = append(patterns,
				config.RedactPattern{Name: "email", Regex: `[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`, ReplaceWith: "[REDACTED_EMAIL]"},
				config.RedactPattern{Name: "phone", Regex: `\+?\d[\d\s().-]{7,}\d`, ReplaceWith: "[REDACTED_PHONE]"},
				config.RedactPattern{Name: "ssn", Regex: `\b\d{3}-\d{2}-\d{4}\b`, ReplaceWith: "[REDACTED_SSN]"},
			)
		case "secrets":
			patterns = append(patterns,
				config.RedactPattern{Name: "api_key", Regex: `(?i)(api[-_ ]?key|secret|token)[^\n\r]*`, ReplaceWith: "[REDACTED_SECRET]"},
			)
		}
	}
	return patterns
}
