package util

import (
	"bytes"
	"encoding/json"
	"sort"
)

type orderedMap struct {
	keys   []string
	values map[string]any
}

func (m orderedMap) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteByte('{')
	for i, key := range m.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyJSON, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		buf.Write(keyJSON)
		buf.WriteByte(':')
		valueJSON, err := json.Marshal(m.values[key])
		if err != nil {
			return nil, err
		}
		buf.Write(valueJSON)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func CanonicalJSON(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}

	ordered := orderJSON(decoded)
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(ordered); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func orderJSON(value any) any {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		ordered := orderedMap{keys: keys, values: make(map[string]any, len(v))}
		for _, key := range keys {
			ordered.values[key] = orderJSON(v[key])
		}
		return ordered
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = orderJSON(item)
		}
		return out
	default:
		return value
	}
}
