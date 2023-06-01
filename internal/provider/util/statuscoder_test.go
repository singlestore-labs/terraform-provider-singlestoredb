package util_test

import (
	"errors"
	"net/http"
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
	require.NotNil(t, result)
	require.Contains(t, result.Detail, string(input.Body))

	result = util.StatusOK(&input, nil)
	require.NotNil(t, result)
	require.Contains(t, result.Detail, string(input.Body), "should deref pointer")

	ierr := errors.New("foo")
	result = util.StatusOK(nil, ierr)
	require.NotNil(t, result)
	require.Contains(t, result.Detail, ierr.Error())

	result = util.StatusOK(management.GetV1RegionsResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusNotFound},
	}, nil)
	require.NotNil(t, result)

	result = util.StatusOK(management.GetV1RegionsResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusNotFound},
	}, nil, util.ReturnNilOnNotFound)
	require.Nil(t, result)

	result = util.StatusOK(management.GetV1RegionsResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusInternalServerError},
	}, nil, util.ReturnNilOnNotFound)
	require.NotNil(t, result)

	result = util.StatusOK(management.GetV1RegionsResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusOK},
	}, nil)
	require.Nil(t, result)
}

type statusCoderNotStruct int

func (sc statusCoderNotStruct) StatusCode() int {
	return int(sc)
}

type statusCoderWithoutBody struct {
	Code int
}

func (sc statusCoderWithoutBody) StatusCode() int {
	return sc.Code
}

func TestMaybeBody(t *testing.T) {
	require.Empty(t, util.MaybeBody(statusCoderNotStruct(0)))
	require.Empty(t, util.MaybeBody(statusCoderWithoutBody{Code: 0}))
	body := "buzz"
	require.Equal(t, body, util.MaybeBody(management.GetV1RegionsResponse{
		Body: []byte(body),
	}))
}
