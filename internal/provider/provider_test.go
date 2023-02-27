package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/terraform-provider-singlestore/examples"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/testutil"
)

func TestProvider(t *testing.T) {
	testutil.UnitTest(t, testutil.Config{}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.Regions,
			},
		},
	})
}
