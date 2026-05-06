package projects_test

import (
	"fmt"
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
	"github.com/zclconf/go-cty/cty"
)

func TestReadsProjectByID(t *testing.T) {
	project := management.Project{
		ProjectID: uuid.MustParse("ad2eb3f8-ef7c-4eb5-b530-6f0930db9ff8"),
		Name:      "main-project",
		Edition:   management.ENTERPRISE,
		CreatedAt: time.Date(2024, time.January, 15, 9, 10, 11, 0, time.UTC),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, fmt.Sprintf("/v1/projects/%s", project.ProjectID), r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(project))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.ProjectGetDataSource).
					WithProjectGetDataSource("this")(config.IDAttribute, cty.StringVal(project.ProjectID.String())).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_project.this", config.IDAttribute, project.ProjectID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_project.this", "name", project.Name),
					resource.TestCheckResourceAttr("data.singlestoredb_project.this", "edition", string(project.Edition)),
					resource.TestCheckResourceAttr("data.singlestoredb_project.this", "created_at", project.CreatedAt.String()),
				),
			},
		},
	})
}

func TestReadProjectByIDError(t *testing.T) {
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
				Config:      examples.ProjectGetDataSource,
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusUnauthorized)),
			},
		},
	})
}

func TestReadProjectByIDInvalidUUID(t *testing.T) {
	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: "http://127.0.0.1:65535",
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.ProjectGetDataSource).
					WithProjectGetDataSource("this")(config.IDAttribute, cty.StringVal("invalid-uuid")).
					String(),
				ExpectError: regexp.MustCompile("invalid UUID"),
			},
		},
	})
}

func TestReadProjectByIDNotFoundIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.ProjectGetDataSource).
					WithProjectGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)),
			},
		},
	})
}

func TestCreateProjectThenGetIntegration(t *testing.T) {
	// API validates project name length 1–35; avoid long names from GenerateUniqueResourceName.
	uniqueName := "p" + strings.ReplaceAll(uuid.New().String(), "-", "")

	cfg := strings.TrimSpace(fmt.Sprintf(`
provider "singlestoredb" {
}

resource "singlestoredb_project" "created" {
  name    = %q
  edition = "STANDARD"
}

data "singlestoredb_project" "fetched" {
  id = singlestoredb_project.created.id
}
`, uniqueName))

	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("singlestoredb_project.created", config.IDAttribute),
					resource.TestCheckResourceAttr("singlestoredb_project.created", "name", uniqueName),
					resource.TestCheckResourceAttr("singlestoredb_project.created", "edition", "STANDARD"),
					resource.TestCheckResourceAttrPair("data.singlestoredb_project.fetched", config.IDAttribute, "singlestoredb_project.created", config.IDAttribute),
					resource.TestCheckResourceAttrPair("data.singlestoredb_project.fetched", "name", "singlestoredb_project.created", "name"),
					resource.TestCheckResourceAttrPair("data.singlestoredb_project.fetched", "edition", "singlestoredb_project.created", "edition"),
					resource.TestCheckResourceAttrPair("data.singlestoredb_project.fetched", "created_at", "singlestoredb_project.created", "created_at"),
				),
			},
		},
	})
}
