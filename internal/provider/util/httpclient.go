package util

import (
	"net/http"

	"github.com/hashicorp/go-retryablehttp"
)

// NewHTTPClient creates an HTTP client for the Terraform provider.
func NewHTTPClient() *http.Client {
	result := retryablehttp.NewClient()

	return result.StandardClient()
}
