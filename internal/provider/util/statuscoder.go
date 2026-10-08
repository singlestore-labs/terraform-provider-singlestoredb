package util

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

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
		return &SummaryWithDetailError{
			Summary: fmt.Sprintf("SingleStore API client returned status code %s", http.StatusText(code)),
			Detail:  unsuccessfulStatusDetail(code, MaybeBody(resp)),
		}
	}

	return nil
}

func unsuccessfulStatusDetail(code int, body string) string {
	detail := "An unsuccessful status code occurred when calling SingleStore API."
	if hint := statusHint(code, body); hint != "" {
		detail += "\n" + hint
	}
	detail += "\n" + config.CreateProviderIssueIfNotClearErrorDetail + "\n\nSingleStore client response body: " + body

	return detail
}

// statusHint adds guidance for statuses that are easy to misread.
// The API key hint is only for 401. A 403 is a credits problem only when the
// body says so; an authorization failure such as "Access to organization is not
// authorized" gets an access hint instead.
func statusHint(code int, body string) string {
	switch code {
	case http.StatusUnauthorized:
		return config.InvalidAPIKeyErrorDetail
	case http.StatusForbidden:
		return forbiddenHint(body)
	default:
		return ""
	}
}

func forbiddenHint(body string) string {
	normalized := strings.ToLower(body)
	switch {
	case indicatesAccessDenied(normalized):
		return config.AccessNotAuthorizedErrorDetail
	case indicatesInsufficientCredits(normalized):
		return config.CreditsErrorDetail
	default:
		return ""
	}
}

func indicatesInsufficientCredits(body string) bool {
	return strings.Contains(body, "credit") ||
		strings.Contains(body, "no active plan") ||
		strings.Contains(body, "billing")
}

func indicatesAccessDenied(body string) bool {
	return strings.Contains(body, "not authorized") ||
		strings.Contains(body, "access denied") ||
		strings.Contains(body, "permission denied")
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
