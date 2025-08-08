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
				config.CreateProviderIssueIfNotClearErrorDetail +
				"\n\nSingleStore client error: " + ierr.Error(),
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
		detail := "An unsuccessful status code occurred when calling SingleStore API. "
		if code == http.StatusUnauthorized || code == http.StatusForbidden {
			detail += config.InvalidAPIKeyErrorDetail
		}
		detail += config.CreateProviderIssueIfNotClearErrorDetail + "\n\nSingleStore client response body: " + MaybeBody(resp)

		return &SummaryWithDetailError{
			Summary: fmt.Sprintf("SingleStore API client returned status code %s", http.StatusText(code)),
			Detail:  detail,
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

func MaybeBody(resp StatusCoder) string {
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
