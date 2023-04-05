package util_test

import (
	"testing"

	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestStatusOK(t *testing.T) {
	input := management.GetV1RegionsResponse{
		Body: []byte("foo-bar-buzz-yes"),
	}
	result := util.StatusOK(input, nil)
	require.Contains(t, result.Detail, string(input.Body))

	result = util.StatusOK(&input, nil)
	require.Contains(t, result.Detail, string(input.Body), "should deref pointer")
}
