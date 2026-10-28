package testutil_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestWithWorkspaceGroupGetDataSoure(t *testing.T) {
	uc := testutil.UpdatableConfig(`invalid`)
	require.Panics(t, func() { _ = uc.WithWorkspaceGroupGetDataSource("this")("foo", cty.StringVal("bar")) })
	uc = testutil.UpdatableConfig(`data "singlestoredb_workspace_group" "this" {
	}`)
	require.Panics(t, func() { _ = uc.WithWorkspaceGroupGetDataSource("no_such_group")("foo", cty.StringVal("bar")) })
	require.NotContains(t, uc, config.IDAttribute)
	id := uuid.New().String()
	uc = uc.WithWorkspaceGroupGetDataSource("this")(config.IDAttribute, cty.StringVal(id))
	require.Contains(t, uc, config.IDAttribute)
	require.Contains(t, uc, id)
}

func TestWithWorkspaceGetDataSoure(t *testing.T) {
	uc := testutil.UpdatableConfig(`data "singlestoredb_workspace" "this" {
	}`)
	require.NotContains(t, uc, config.IDAttribute)
	id := uuid.New().String()
	uc = uc.WithWorkspaceGetDataSource("this")(config.IDAttribute, cty.StringVal(id))
	require.Contains(t, uc, config.IDAttribute)
	require.Contains(t, uc, id)
}

func TestWithWorkspaceListDataSoure(t *testing.T) {
	uc := testutil.UpdatableConfig(`data "singlestoredb_workspaces" "this" {
	}`)
	require.NotContains(t, uc, config.IDAttribute)
	id := uuid.New().String()
	uc = uc.WithWorkspaceListDataSource("this")(config.IDAttribute, cty.StringVal(id))
	require.Contains(t, uc, config.IDAttribute)
	require.Contains(t, uc, id)
}

func TestWithWorkspaceResource(t *testing.T) {
	uc := testutil.UpdatableConfig(`resource "singlestoredb_workspace" "example" {
	}`)
	require.NotContains(t, uc, "suspended")
	uc = uc.WithWorkspaceResource("example")("suspended", cty.BoolVal(true))
	require.Contains(t, uc, "suspended")
	require.Contains(t, uc, "true")
	uc = uc.WithWorkspaceResource("example")("suspended", cty.BoolVal(false))
	require.NotContains(t, uc, "true")
	require.Contains(t, uc, "false")
}

func TestWithWorkspaceGroupResource(t *testing.T) {
	uc := testutil.UpdatableConfig(`resource "singlestoredb_workspace_group" "this" {
	}`)
	require.NotContains(t, uc, config.IDAttribute)
	id := uuid.New().String()
	uc = uc.WithWorkspaceGroupResource("this")(config.IDAttribute, cty.StringVal(id))
	require.Contains(t, uc, config.IDAttribute)
	require.Contains(t, uc, id)
}

func TestWithIntegrationSafeguards(t *testing.T) {
	expiresAt := "2026-10-28T14:00:00Z"
	original := testutil.UpdatableConfig(`
resource "singlestoredb_workspace_group" "example" {
  name       = "group"
  expires_at = "2222-01-01T00:00:00Z"
}

resource "singlestoredb_workspace" "this" {
  name = "workspace-1"
}

resource "singlestoredb_workspace" "reader" {
  name = "workspace-2"
}

resource "singlestoredb_cluster" "this" {
  name       = "cluster-1"
  expires_at = "2222-01-01T00:00:00Z"
}
`)

	got := original.WithIntegrationSafeguards(expiresAt).String()

	require.NotContains(t, got, "2222-01-01T00:00:00Z")
	require.NotContains(t, got, config.TestWorkspaceName)
	require.Contains(t, got, config.TestReaderWorkspaceName)
	require.Contains(t, got, expiresAt)
	require.Equal(t, 2, strings.Count(got, config.TestClusterName))

	untouched := testutil.UpdatableConfig(`data "singlestoredb_regions" "all" {}`)
	require.Equal(t, untouched.String(), untouched.WithIntegrationSafeguards(expiresAt).String())
}

func TestWithAPIKey(t *testing.T) {
	uc := testutil.UpdatableConfig(`provider "singlestoredb" {
	}`)
	require.Equal(t, uc, uc.WithAPIKey(""), "an empty API key changes nothing")
	apiKey := "abc"
	require.NotContains(t, uc, config.APIKeyAttribute)
	uc = uc.WithAPIKey(apiKey)
	require.Contains(t, uc, config.APIKeyAttribute)
	require.Contains(t, uc, apiKey)
}

func TestWithAPIKeyPath(t *testing.T) {
	uc := testutil.UpdatableConfig(`provider "singlestoredb" {
	}`)
	apiKeyPath := "/foo/bar/abc.txt" //nolint:gosec
	require.NotContains(t, uc, config.APIKeyPathAttribute)
	uc = uc.WithAPIKeyPath(apiKeyPath)
	require.Contains(t, uc, config.APIKeyPathAttribute)
	require.Contains(t, uc, apiKeyPath)
}

func TestWithAPIServiceURL(t *testing.T) {
	uc := testutil.UpdatableConfig(`provider "singlestoredb" {
	}`)
	require.Equal(t, uc.String(), uc.WithAPIServiceURL("").String(), "an empty URL changes nothing")
	url := "localhost:8888"
	require.NotContains(t, uc, config.APIServiceURLAttribute)
	uc = uc.WithAPIServiceURL(url)
	require.Contains(t, uc, config.APIServiceURLAttribute)
	require.Contains(t, uc, url)
}
