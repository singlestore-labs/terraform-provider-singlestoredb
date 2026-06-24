package sql_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/sql"
	"github.com/stretchr/testify/require"
)

func TestStringifyRow(t *testing.T) {
	t.Parallel()

	row := map[string]any{
		"name":   "alice",
		"active": true,
		"count":  json.Number("42"),
		"meta":   map[string]any{"k": "v"},
		"nilcol": nil,
	}

	got, err := sql.StringifyRows([]map[string]any{row})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "alice", got[0]["name"])
	require.Equal(t, "true", got[0]["active"])
	require.Equal(t, "42", got[0]["count"])
	require.Equal(t, `{"k":"v"}`, got[0]["meta"])
	require.Equal(t, "", got[0]["nilcol"])
}

func TestStringifyRow_ByteSlice(t *testing.T) {
	t.Parallel()

	got, err := sql.StringifyRows([]map[string]any{{"blob": []byte("raw bytes")}})
	require.NoError(t, err)
	require.Equal(t, "raw bytes", got[0]["blob"])
}

func TestStringifyRow_FloatAndNested(t *testing.T) {
	t.Parallel()

	got, err := sql.StringifyRows([]map[string]any{{
		"price": 3.14,
		"tags":  []any{"a", "b"},
	}})
	require.NoError(t, err)
	require.Equal(t, "3.14", got[0]["price"])
	require.Equal(t, `["a","b"]`, got[0]["tags"])
}

func TestStringifyRow_MarshalError(t *testing.T) {
	t.Parallel()

	// A channel is not JSON-serializable, exercising the json.Marshal error branch.
	_, err := sql.StringifyRows([]map[string]any{{"bad": make(chan int)}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "bad")
}

func TestStringifyRow_BigIntPrecision(t *testing.T) {
	t.Parallel()

	var row map[string]any
	dec := json.NewDecoder(strings.NewReader(`{"id":9223372036854775807}`))
	dec.UseNumber()
	require.NoError(t, dec.Decode(&row))

	got, err := sql.StringifyRows([]map[string]any{row})
	require.NoError(t, err)
	require.Equal(t, "9223372036854775807", got[0]["id"])
}

func TestFirstResultSetRows(t *testing.T) {
	t.Parallel()

	require.Nil(t, sql.FirstResultSetRowsForTest(nil))
	require.Nil(t, sql.FirstResultSetRowsForTest(&sql.QueryRowsResponse{}))

	var resp sql.QueryRowsResponse
	require.NoError(t, json.Unmarshal([]byte(`{"results":[{"rows":[{"a":"1"}]}]}`), &resp))
	require.Len(t, sql.FirstResultSetRowsForTest(&resp), 1)
}

func TestStringArgsToAny(t *testing.T) {
	t.Parallel()

	got := sql.StringArgsToAny([]string{"a", "b"})
	require.Len(t, got, 2)
	require.Equal(t, "a", got[0])
	require.Equal(t, "b", got[1])
}

func TestStringifyRows_FirstResultSetOnlyViaQueryResponse(t *testing.T) {
	t.Parallel()

	var resp sql.QueryRowsResponse
	require.NoError(t, json.Unmarshal([]byte(`{
		"results":[
			{"rows":[{"a":"1"}]},
			{"rows":[{"a":"2"}]}
		]
	}`), &resp))

	// QueryRows decodes with UseNumber in client; for this unit test we only
	// stringify the first result set as the resource/data source will.
	require.Len(t, resp.Results, 2)

	first := resp.Results[0].Rows
	got, err := sql.StringifyRows(first)
	require.NoError(t, err)
	require.Equal(t, "1", got[0]["a"])
}
