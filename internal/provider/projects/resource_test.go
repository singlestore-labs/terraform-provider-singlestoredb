package projects_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/examples"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/stretchr/testify/require"
)

const projectsPath = "/v1/projects"

var testProject = management.Project{
	ProjectID: uuid.MustParse("ad2eb3f8-ef7c-4eb5-b530-6f0930db9ff8"),
	Name:      "project",
	Edition:   management.STANDARD,
	CreatedAt: time.Date(2024, time.January, 15, 9, 10, 11, 0, time.UTC),
}

func TestCRUDProject(t *testing.T) {
	project := testProject

	projectPostHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, projectsPath, r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var input management.ProjectCreate
		require.NoError(t, json.Unmarshal(body, &input))
		require.Equal(t, project.Name, input.Name)
		require.Equal(t, project.Edition, input.Edition)

		w.Header().Add("Content-Type", "application/json")
		_, err = w.Write(testutil.MustJSON(management.ProjectIDResponse{
			ProjectID: project.ProjectID,
		}))
		require.NoError(t, err)
	}

	projectGetPath := strings.Join([]string{projectsPath, project.ProjectID.String()}, "/")

	projectGetHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != projectGetPath || r.Method != http.MethodGet {
			return false
		}

		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(project))
		require.NoError(t, err)

		return true
	}

	projectPatchHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{projectsPath, project.ProjectID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodPatch, r.Method)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var input management.ProjectUpdate
		require.NoError(t, json.Unmarshal(body, &input))

		project.Name = input.Name

		w.Header().Add("Content-Type", "application/json")
		_, err = w.Write(testutil.MustJSON(management.ProjectIDResponse{
			ProjectID: project.ProjectID,
		}))
		require.NoError(t, err)
	}

	projectDeleteHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{projectsPath, project.ProjectID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)

		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(management.ProjectIDResponse{
			ProjectID: project.ProjectID,
		}))
		require.NoError(t, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == projectsPath && r.Method == http.MethodPost:
			projectPostHandler(w, r)
		case projectGetHandler(w, r):
			// handled
		case r.Method == http.MethodPatch:
			projectPatchHandler(w, r)
		case r.Method == http.MethodDelete:
			projectDeleteHandler(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.ProjectResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_project.this", config.IDAttribute, testProject.ProjectID.String()),
					resource.TestCheckResourceAttr("singlestoredb_project.this", "name", testProject.Name),
					resource.TestCheckResourceAttr("singlestoredb_project.this", "edition", string(testProject.Edition)),
					resource.TestCheckResourceAttr("singlestoredb_project.this", "created_at", testProject.CreatedAt.String()),
				),
			},
			{
				Config: `
provider "singlestoredb" {
}

resource "singlestoredb_project" "this" {
  name    = "updated-project"
  edition = "STANDARD"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_project.this", config.IDAttribute, testProject.ProjectID.String()),
					resource.TestCheckResourceAttr("singlestoredb_project.this", "name", "updated-project"),
					resource.TestCheckResourceAttr("singlestoredb_project.this", "edition", string(testProject.Edition)),
				),
			},
		},
	})
}

func TestCreateProjectError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        "bar",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:      examples.ProjectResource,
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusUnauthorized)),
			},
		},
	})
}

func TestImportProject(t *testing.T) {
	project := testProject

	projectGetPath := strings.Join([]string{projectsPath, project.ProjectID.String()}, "/")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == projectsPath && r.Method == http.MethodPost:
			w.Header().Add("Content-Type", "application/json")
			_, err := w.Write(testutil.MustJSON(management.ProjectIDResponse{
				ProjectID: project.ProjectID,
			}))
			require.NoError(t, err)
		case r.URL.Path == projectGetPath && r.Method == http.MethodGet:
			w.Header().Add("Content-Type", "application/json")
			_, err := w.Write(testutil.MustJSON(project))
			require.NoError(t, err)
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.ProjectResource,
			},
			{
				ResourceName:      "singlestoredb_project.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestUpdateEditionNotAllowed(t *testing.T) {
	project := testProject

	projectGetPath := strings.Join([]string{projectsPath, project.ProjectID.String()}, "/")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == projectsPath && r.Method == http.MethodPost:
			w.Header().Add("Content-Type", "application/json")
			_, err := w.Write(testutil.MustJSON(management.ProjectIDResponse{
				ProjectID: project.ProjectID,
			}))
			require.NoError(t, err)
		case r.URL.Path == projectGetPath && r.Method == http.MethodGet:
			w.Header().Add("Content-Type", "application/json")
			_, err := w.Write(testutil.MustJSON(project))
			require.NoError(t, err)
		case r.Method == http.MethodDelete:
			w.Header().Add("Content-Type", "application/json")
			_, err := w.Write(testutil.MustJSON(management.ProjectIDResponse{
				ProjectID: project.ProjectID,
			}))
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.ProjectResource,
			},
			{
				Config: `
provider "singlestoredb" {
}

resource "singlestoredb_project" "this" {
  name    = "project"
  edition = "ENTERPRISE"
}
`,
				ExpectError: regexp.MustCompile("Cannot update edition"),
			},
		},
	})
}

func TestCRUDProjectIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.ProjectResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("singlestoredb_project.this", config.IDAttribute),
					resource.TestCheckResourceAttr("singlestoredb_project.this", "name", "project"),
					resource.TestCheckResourceAttr("singlestoredb_project.this", "edition", "STANDARD"),
					resource.TestCheckResourceAttrSet("singlestoredb_project.this", "created_at"),
				),
			},
		},
	})
}
