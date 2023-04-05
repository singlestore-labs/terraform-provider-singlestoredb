package util

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
)

type StatusCoder interface {
	StatusCode() int
}

type StatusOKOption func(code int) (overrideReturn bool, newResult *SummaryWithDetailError)

func StatusOK(resp StatusCoder, ierr error,
	opts ...StatusOKOption,
) *SummaryWithDetailError {
	if ierr != nil {
		return &SummaryWithDetailError{
			Summary: "SingleStore API client call failed",
			Detail: "An unexpected error occurred when calling SingleStore API. " +
				"If the error is not clear, please contact the provider developers.\n\n" +
				"SingleStore client error: " + ierr.Error(),
		}
	}

	code := resp.StatusCode()

	for _, opt := range opts {
		overrideReturn, newResult := opt(code)
		if overrideReturn {
			return newResult
		}
	}

	if code != http.StatusOK {
		return &SummaryWithDetailError{
			Summary: fmt.Sprintf("SingleStore API client returned status code %s", http.StatusText(code)),
			Detail: "An unsuccessful status code occurred when calling SingleStore API. " +
				fmt.Sprintf("Make sure to set the %s value in the configuration or use the %s environment variable. ", config.APIKeyAttribute, config.EnvAPIKey) +
				"If the error is not clear, please contact the provider developers.\n\n" +
				"SingleStore client response body: " + maybeBody(resp),
		}
	}

	return nil
}

func ReturnNilOnNotFound(code int) (bool, *SummaryWithDetailError) {
	if code == http.StatusNotFound {
		return true, nil
	}

	return false, nil
}

func maybeBody(resp StatusCoder) string {
	v := reflect.ValueOf(resp)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return ""
	}

	bodyField := v.FieldByName("Body")
	if !bodyField.IsValid() || bodyField.Type() != reflect.TypeOf([]byte{}) {
		return ""
	}

	result := bodyField.Bytes()

	return string(result)
}
