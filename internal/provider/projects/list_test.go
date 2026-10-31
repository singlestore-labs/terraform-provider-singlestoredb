package projects_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
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

func TestReadsProjects(t *testing.T) {
	projects := []management.Project{
		{
			ProjectID: uuid.MustParse("ad2eb3f8-ef7c-4eb5-b530-6f0930db9ff8"),
			Name:      "main-project",
			Edition:   management.ENTERPRISE,
			CreatedAt: time.Date(2024, time.January, 15, 9, 10, 11, 0, time.UTC),
		},
		{
			ProjectID: uuid.MustParse("7e0f6da7-bf11-42dc-8b57-31e77140fbf3"),
			Name:      "analytics-project",
			Edition:   management.ENTERPRISE,
			CreatedAt: time.Date(2024, time.February, 19, 12, 16, 17, 0, time.UTC),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/projects", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(projects))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.ProjectsListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_projects.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_projects.all", "projects.#", "2"),
					resource.TestCheckResourceAttr("data.singlestoredb_projects.all", "projects.0.id", projects[0].ProjectID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_projects.all", "projects.0.name", projects[0].Name),
					resource.TestCheckResourceAttr("data.singlestoredb_projects.all", "projects.0.edition", string(projects[0].Edition)),
					resource.TestCheckResourceAttr("data.singlestoredb_projects.all", "projects.0.created_at",
						projects[0].CreatedAt.String(),
					),
					resource.TestCheckResourceAttr("data.singlestoredb_projects.all", "projects.1.id", projects[1].ProjectID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_projects.all", "projects.1.name", projects[1].Name),
					resource.TestCheckResourceAttr("data.singlestoredb_projects.all", "projects.1.edition", string(projects[1].Edition)),
					resource.TestCheckResourceAttr("data.singlestoredb_projects.all", "projects.1.created_at",
						projects[1].CreatedAt.String(),
					),
				),
			},
		},
	})
}

func TestReadProjectsError(t *testing.T) {
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
				Config:      examples.ProjectsListDataSource,
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusUnauthorized)),
			},
		},
	})
}

func TestReadsProjectsIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.ProjectsListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_projects.all", config.IDAttribute, config.TestIDValue),
					// Checking that at least no error.
				),
			},
		},
	})
}
