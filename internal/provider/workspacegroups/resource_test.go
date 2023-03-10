package workspacegroups_test

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/terraform-provider-singlestore/examples"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/testutil"
)

var integrationTestUpdatedWorkspaceGroupName = strings.Join([]string{"updated", config.IntegrationTestInitialWorkspaceGroupName}, "-") //nolint

func TestWorkspaceGroupResourceIntegration(t *testing.T) {
	apiKey := os.Getenv(config.EnvTestAPIKey)

	testutil.IntegrationTest(t, apiKey, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.WorkspaceGroupsResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "id", config.TestIDValue),
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "name", config.IntegrationTestInitialWorkspaceGroupName),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsResource).
					WithOverride(
						config.IntegrationTestInitialWorkspaceGroupName,
						integrationTestUpdatedWorkspaceGroupName,
					).String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "id", config.TestIDValue),
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "name", integrationTestUpdatedWorkspaceGroupName),
				),
			},
		},
	})
}
