package util

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ParseJSONMap(input string) (map[string]any, error) {
	if input == "" {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(input), &out); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	return out, nil
}

func ParseJSONValue(input string) (any, error) {
	if strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("empty value")
	}
	var out any
	if err := json.Unmarshal([]byte(input), &out); err == nil {
		return out, nil
	}
	return input, nil
}
