package regions_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestore/examples"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/testutil"
	"github.com/stretchr/testify/require"
)

func TestReadsRegions(t *testing.T) {
	regions := []management.Region{
		{
			RegionID: uuid.MustParse("e495c7f3-b37a-4234-8e8f-f715257e3a6c"),
			Region:   "GS - US West 2 (Oregon) - aws-oregon-gs1",
			Provider: management.AWS,
		},
		{
			RegionID: uuid.MustParse("e8f6f596-6fba-4b87-adb1-7f9e960c7c78"),
			Region:   "East US 1 (Virginia)",
			Provider: management.Azure,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/regions", r.URL.Path)
		w.Header().Add("Content-Type", "json") // Necessary to make the library parse the resulting JSON.
		_, err := w.Write(testutil.MustJSON(regions))
		require.NoError(t, err)
	}))
	defer server.Close()

	testutil.UnitTest(t, testutil.Config{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.Regions,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestore_regions.all", config.TestIDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestore_regions.all", "regions.#", "2"),
					resource.TestCheckResourceAttr("data.singlestore_regions.all", "regions.0.region_id", regions[0].RegionID.String()),
					resource.TestCheckResourceAttr("data.singlestore_regions.all", "regions.0.region", regions[0].Region),
					resource.TestCheckResourceAttr("data.singlestore_regions.all", "regions.0.provider", string(regions[0].Provider)),
					resource.TestCheckResourceAttr("data.singlestore_regions.all", "regions.1.region_id", regions[1].RegionID.String()),
				),
			},
		},
	})
}

func TestReadRegionsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	r := regexp.MustCompile(http.StatusText(http.StatusUnauthorized))

	testutil.UnitTest(t, testutil.Config{
		APIServiceURL: server.URL,
		APIKey:        "bar",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:      examples.Regions,
				ExpectError: r,
			},
		},
	})
}

func TestReadsRegionsIntegration(t *testing.T) {
	apiKey := os.Getenv(config.EnvTestAPIKey)

	testutil.IntegrationTest(t, apiKey, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.Regions,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestore_regions.all", config.TestIDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttrSet("data.singlestore_regions.all", "regions.0.region_id"),
					resource.TestCheckResourceAttrSet("data.singlestore_regions.all", "regions.0.region"),
					resource.TestCheckResourceAttrSet("data.singlestore_regions.all", "regions.0.provider"),
					// Checking that at least 1 element and that element is with the expected fields.
				),
			},
		},
	})
}
