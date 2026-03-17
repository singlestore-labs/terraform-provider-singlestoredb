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

func TestReadProjects(t *testing.T) {
	projects := []management.Project{
		{
			ProjectID: uuid.MustParse("f3ccf58f-8df7-4e19-b8b9-5bb5b9d8d4be"),
			Name:      "Dev Project",
			Edition:   management.ProjectEdition("STANDARD"),
			CreatedAt: time.Date(2026, 1, 14, 13, 21, 17, 0, time.UTC),
		},
		{
			ProjectID: uuid.MustParse("9dc5595f-fbb7-4d7c-b9e8-6ac4f1d8ad0a"),
			Name:      "Prod Project",
			Edition:   management.ProjectEdition("ENTERPRISE"),
			CreatedAt: time.Date(2026, 1, 15, 10, 3, 4, 0, time.UTC),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/projects", r.URL.Path)
		w.Header().Add("Content-Type", "application/json")
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
					resource.TestCheckResourceAttr("data.singlestoredb_projects.all", "projects.0.created_at", projects[0].CreatedAt.Format(time.RFC3339)),
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

func TestReadProjectsIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.ProjectsListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_projects.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttrSet("data.singlestoredb_projects.all", "projects.0.id"),
					resource.TestCheckResourceAttrSet("data.singlestoredb_projects.all", "projects.0.name"),
					resource.TestCheckResourceAttrSet("data.singlestoredb_projects.all", "projects.0.edition"),
				),
			},
		},
	})
}
