package sql

import (
	"encoding/json"
	"fmt"
)

// stringifyRow converts a JSON-decoded row to map[string]string for Terraform state.
func stringifyRow(in map[string]any) (map[string]string, error) {
	out := make(map[string]string, len(in))
	for key, value := range in {
		s, err := stringifyValue(value)
		if err != nil {
			return nil, fmt.Errorf("column %q: %w", key, err)
		}

		out[key] = s
	}

	return out, nil
}

// StringifyRows converts each row in rows to map[string]string.
func StringifyRows(rows []map[string]any) ([]map[string]string, error) {
	result := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		s, err := stringifyRow(row)
		if err != nil {
			return nil, err
		}

		result = append(result, s)
	}

	return result, nil
}

func stringifyValue(value any) (string, error) {
	if value == nil {
		return "", nil
	}

	switch v := value.(type) {
	case string:
		return v, nil
	case bool:
		if v {
			return "true", nil
		}

		return "false", nil
	case json.Number:
		return v.String(), nil
	case []byte:
		return string(v), nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}

		return string(b), nil
	}
}

// StringArgsToAny converts Terraform list(string) args to Data API []any args.
func StringArgsToAny(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}

	return out
}
