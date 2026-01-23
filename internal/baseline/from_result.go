package baseline

import "github.com/regrada-ai/regrada/internal/eval"

func FromResult(result eval.CaseResult, baselineKey, paramsHash string) Baseline {
	b := Baseline{
		CaseID:      result.CaseID,
		BaselineKey: baselineKey,
		Provider:    result.Provider,
		Model:       result.Model,
		ParamsHash:  paramsHash,
		Aggregates: Aggregates{
			PassRate:      result.Aggregates.PassRate,
			LatencyP95MS:  result.Aggregates.LatencyP95MS,
			RefusalRate:   result.Aggregates.RefusalRate,
			JSONValidRate: result.Aggregates.JSONValidRate,
		},
	}

	if len(result.Runs) > 0 {
		b.GoldenText = result.Runs[0].OutputText
		b.GoldenJSON = result.Runs[0].JSON
	}

	return b
}
