package accept

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/regrada-ai/regrada/internal/cases"
	"github.com/regrada-ai/regrada/internal/model"
	"github.com/regrada-ai/regrada/internal/trace"
	"github.com/regrada-ai/regrada/internal/util"
)

type ConvertOptions struct {
	DefaultTags  []string
	InferAsserts bool
	Normalize    NormalizeOptions
}

type NormalizeOptions struct {
	TrimWhitespace     bool
	DropVolatileFields bool
}

func FromTrace(t trace.Trace, opts ConvertOptions) (cases.Case, error) {
	caseID := fmt.Sprintf("recorded.%s", util.ShortHash(t.TraceID))

	messages := t.Request.Messages
	if opts.Normalize.TrimWhitespace {
		for i := range messages {
			messages[i].Content = strings.TrimSpace(messages[i].Content)
		}
	}

	caseTags := append([]string{}, opts.DefaultTags...)
	if t.Provider != "" {
		caseTags = append(caseTags, "provider:"+t.Provider)
	}
	if t.Model != "" {
		caseTags = append(caseTags, "model:"+t.Model)
	}

	c := cases.Case{
		ID:   caseID,
		Tags: caseTags,
		Request: cases.CaseRequest{
			Messages: messages,
			Params:   convertParams(t.Request.Params),
		},
	}

	if opts.InferAsserts {
		c.Assert = inferAsserts(t.Response.AssistantText)
	}

	return c, nil
}

func inferAsserts(output string) *cases.CaseAssert {
	assert := &cases.CaseAssert{}
	if output != "" {
		maxChars := int(math.Ceil(float64(len(output)) * 1.5))
		assert.Text = &cases.TextAssert{MaxChars: &maxChars}
	}

	var tmp any
	if output != "" && json.Unmarshal([]byte(output), &tmp) == nil {
		if assert.JSON == nil {
			assert.JSON = &cases.JSONAssert{}
		}
		v := true
		assert.JSON.Valid = &v
	}

	if assert.Text == nil && assert.JSON == nil {
		return nil
	}
	return assert
}

func convertParams(params *model.SamplingParams) *cases.CaseParams {
	if params == nil {
		return nil
	}
	return &cases.CaseParams{
		Temperature:     params.Temperature,
		TopP:            params.TopP,
		MaxOutputTokens: params.MaxOutputTokens,
		Stop:            params.Stop,
	}
}
