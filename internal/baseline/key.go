package baseline

import (
	"encoding/json"
	"fmt"

	"github.com/regrada-ai/regrada/internal/model"
	"github.com/regrada-ai/regrada/internal/util"
)

func Key(provider, model string, params *model.SamplingParams, systemPrompt string) (string, string, error) {
	payload := map[string]any{
		"provider": provider,
		"model":    model,
		"params":   params,
		"system":   systemPrompt,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}
	hash := util.ShortHash(string(data))
	slugModel := util.Slugify(model)
	if slugModel == "" {
		slugModel = "model"
	}
	key := fmt.Sprintf("%s-%s-%s", provider, slugModel, hash)
	return key, hash, nil
}
