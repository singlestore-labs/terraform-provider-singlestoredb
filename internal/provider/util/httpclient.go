package util

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/go-retryablehttp"
)

const respReadLimit = int64(4096)

// NewHTTPClient creates an HTTP client for the Terraform provider.
func NewHTTPClient() *http.Client {
	result := retryablehttp.NewClient()
	result.ErrorHandler = HandleError

	return result.StandardClient()
}

var _ retryablehttp.ErrorHandler = HandleError

// HandleError overrides the default behavior of the library
// by exposing the underlying issue because the underlying issue may be useful, e.g.,
// a customer running out of credits and still closing the body.
//
// This function is called if retries are expired, containing the last status
// from the http library. If not specified, default behavior for the library is
// to close the body and return an error indicating how many tries were attempted.
//
// The function is called only when server returns 500s.
func HandleError(resp *http.Response, ierr error, numTries int) (*http.Response, error) {
	if resp == nil {
		return nil, maybeWithExtraError(fmt.Sprintf("giving up after %d attempts, unable to read response body", numTries), ierr)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, respReadLimit))
	if err != nil {
		result := fmt.Sprintf("giving up after %d attempts, unable to read response body, status code: %s, error: %s", numTries, http.StatusText(resp.StatusCode), err)

		return nil, maybeWithExtraError(result, ierr)
	}

	result := fmt.Sprintf("giving up after %d attempts, unexpected status code: %s, response: %s", numTries, http.StatusText(resp.StatusCode), body)

	return nil, maybeWithExtraError(result, ierr)
}

func maybeWithExtraError(main string, extra error) error {
	if extra == nil {
		return errors.New(main)
	}

	return fmt.Errorf("%s: %w", main, extra)
}
