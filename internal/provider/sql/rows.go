package sql

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// QueryResultsElementType is the Terraform type for query_results rows.
var QueryResultsElementType = types.MapType{ElemType: types.StringType}

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

// RowsToTFList converts stringified rows to a Terraform list(map(string)).
func RowsToTFList(ctx context.Context, rows []map[string]string) (types.List, diag.Diagnostics) {
	elements := make([]attr.Value, 0, len(rows))
	for _, row := range rows {
		elem, diags := types.MapValueFrom(ctx, types.StringType, row)
		if diags.HasError() {
			return types.ListNull(QueryResultsElementType), diags
		}

		elements = append(elements, elem)
	}

	list, diags := types.ListValue(QueryResultsElementType, elements)
	if diags.HasError() {
		return types.ListNull(QueryResultsElementType), diags
	}

	return list, nil
}

// EmptyQueryResults returns an empty query_results list.
func EmptyQueryResults() types.List {
	return types.ListValueMust(QueryResultsElementType, []attr.Value{})
}

// firstResultSetRows returns results[0].rows or nil when empty.
func firstResultSetRows(resp *QueryRowsResponse) []map[string]any {
	if resp == nil || len(resp.Results) == 0 {
		return nil
	}

	return resp.Results[0].Rows
}

// StringArgsToAny converts Terraform list(string) args to Data API []any args.
func StringArgsToAny(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}

	return out
}

// ListStrings extracts list(string) values from a Terraform list.
func ListStrings(ctx context.Context, list types.List) ([]string, diag.Diagnostics) {
	if list.IsNull() || list.IsUnknown() {
		return nil, nil
	}

	var result []string
	diags := list.ElementsAs(ctx, &result, false)

	return result, diags
}
