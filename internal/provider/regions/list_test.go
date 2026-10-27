package regions_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/examples"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/stretchr/testify/require"
)

func TestReadsRegions(t *testing.T) {
	regions := []management.RegionV2{
		{
			Region:     "GS - US West 2 (Oregon) - aws-oregon-gs1",
			Provider:   management.CloudProviderAWS,
			RegionName: "aws-oregon-gs1",
		},
		{
			Region:     "East US 1 (Virginia)",
			Provider:   management.CloudProviderAzure,
			RegionName: "eastus",
		},
	}

	legacyRegions := []management.Region{
		{
			RegionID: uuid.MustParse("e495c7f3-b37a-4234-8e8f-f715257e3a6c"),
			Region:   regions[0].Region,
			Provider: regions[0].Provider,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json") // Necessary to make the library parse the resulting JSON.

		var body []byte
		switch r.URL.Path {
		case "/v2/regions":
			body = testutil.MustJSON(regions)
		case "/v1/regions":
			body = testutil.MustJSON(legacyRegions)
		default:
			require.Failf(t, "unexpected path", "path: %s", r.URL.Path)
		}

		_, err := w.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.Regions,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_regions.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_regions.all", "regions.#", "2"),
					resource.TestCheckResourceAttr("data.singlestoredb_regions.all", "regions.0.region", regions[0].Region),
					resource.TestCheckResourceAttr("data.singlestoredb_regions.all", "regions.0.provider", string(regions[0].Provider)),
					resource.TestCheckResourceAttr("data.singlestoredb_regions.all", "regions.0.region_name", regions[0].RegionName),
					resource.TestCheckResourceAttr("data.singlestoredb_regions.all", fmt.Sprintf("regions.0.%s", config.IDAttribute), legacyRegions[0].RegionID.String()),
					resource.TestCheckNoResourceAttr("data.singlestoredb_regions.all", fmt.Sprintf("regions.1.%s", config.IDAttribute)),
				),
			},
		},
	})
}

func TestReadRegionsError(t *testing.T) {
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
				Config:      examples.Regions,
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusUnauthorized)),
			},
		},
	})
}

func TestReadsRegionsIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.Regions,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_regions.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttrSet("data.singlestoredb_regions.all", "regions.0.region"),
					resource.TestCheckResourceAttrSet("data.singlestoredb_regions.all", "regions.0.provider"),
					resource.TestCheckResourceAttrSet("data.singlestoredb_regions.all", "regions.0.region_name"),
					// Checking that at least 1 element and that element is with the expected fields.
				),
			},
		},
	})
}
