package regions_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json") // Necessary to make the library parse the resulting JSON.

		var body []byte
		switch r.URL.Path {
		case "/v2/regions":
			body = testutil.MustJSON(regions)
		case "/v1/regions":
			require.Fail(t, "singlestoredb_regions must not call the v1 regions API")

			return
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
					resource.TestCheckNoResourceAttr("data.singlestoredb_regions.all", "regions.0.id"),
					resource.TestCheckNoResourceAttr("data.singlestoredb_regions.all", "regions.1.id"),
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
