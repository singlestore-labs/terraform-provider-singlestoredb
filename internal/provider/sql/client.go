package sql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	// MaxRequestBodyBytes is the Data API request body size limit.
	MaxRequestBodyBytes = 1 << 20

	execPath       = "/api/v2/exec"
	queryRowsPath  = "/api/v2/query/rows"
	defaultVersion = "dev"
)

var providerVersion = defaultVersion

// SetProviderVersion sets the User-Agent version segment for Data API requests.
func SetProviderVersion(version string) {
	if version == "" {
		providerVersion = defaultVersion

		return
	}

	providerVersion = version
}

// Client calls the SingleStore Data API over HTTPS.
type Client struct {
	httpClient *http.Client
	baseURL    string
	host       string
	username   string
	password   string
}

// ExecRequest is the JSON body for /exec and /query/rows.
type ExecRequest struct {
	SQL      string `json:"sql"`
	Args     []any  `json:"args,omitempty"`
	Database string `json:"database,omitempty"`
}

// ExecResponse is the JSON body from /api/v2/exec.
type ExecResponse struct {
	LastInsertID int64 `json:"lastInsertId"`
	RowsAffected int64 `json:"rowsAffected"`
}

// QueryRowsResponse is the JSON body from /api/v2/query/rows.
type QueryRowsResponse struct {
	Results []struct {
		Rows []map[string]any `json:"rows"`
	} `json:"results"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// NewClient creates a Data API client for the given base URL and credentials.
func NewClient(baseURL, username, password string) *Client {
	return &Client{
		httpClient: util.NewHTTPClient(),
		baseURL:    baseURL,
		host:       HostFromDataAPIURL(baseURL),
		username:   username,
		password:   password,
	}
}

// Exec runs a statement via POST /api/v2/exec.
func (c *Client) Exec(ctx context.Context, req ExecRequest) (*ExecResponse, error) {
	body, err := c.postJSON(ctx, execPath, req)
	if err != nil {
		return nil, err
	}

	var result ExecResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode exec response: %w", err)
	}

	return &result, nil
}

// QueryRows runs a read query via POST /api/v2/query/rows.
func (c *Client) QueryRows(ctx context.Context, req ExecRequest) (*QueryRowsResponse, error) {
	body, err := c.postJSON(ctx, queryRowsPath, req)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	var result QueryRowsResponse
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("decode query response: %w", err)
	}

	if result.Error != nil {
		return nil, &QueryError{
			Message: result.Error.Message,
			Host:    c.host,
		}
	}

	return &result, nil
}

func (c *Client) postJSON(ctx context.Context, path string, payload ExecRequest) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	if len(body) > MaxRequestBodyBytes {
		return nil, RequestTooLargeError{}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", util.TerraformProviderUserAgent(providerVersion))
	httpReq.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
			Host:       c.host,
		}
	}

	return respBody, nil
}
