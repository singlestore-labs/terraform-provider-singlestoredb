package workspacegroups

import (
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestIsFatalWorkspaceGroupCreationError(t *testing.T) {
	require.False(t, isFatalWorkspaceGroupCreationError(nil))
	require.False(t, isFatalWorkspaceGroupCreationError(&util.SummaryWithDetailError{
		Summary: "other",
		Detail:  "workspace group is not ready yet",
	}))
	require.True(t, isFatalWorkspaceGroupCreationError(&util.SummaryWithDetailError{
		Summary: "Failed to wait",
		Detail:  "Workspace group is not ready: workspace group x creation failed (state FAILED); contact support",
	}))
}

func TestFatalWorkspaceGroupStateErrorMessage(t *testing.T) {
	id := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	err := fatalWorkspaceGroupStateError{
		id:    id,
		state: management.WorkspaceGroupStateFAILED,
	}
	require.Contains(t, err.Error(), id.String())
	require.Contains(t, err.Error(), string(management.WorkspaceGroupStateFAILED))
	require.Contains(t, err.Error(), "creation failed (state")
}

func TestResolveCreateIDsWithRegionID(t *testing.T) {
	regionID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	plan := workspaceGroupResourceModel{
		RegionID: types.StringValue(regionID.String()),
	}

	gotRegionID, gotProjectID, err := resolveCreateIDs(t.Context(), nil, plan)
	require.Nil(t, err)
	require.Nil(t, gotProjectID)
	require.NotNil(t, gotRegionID)
	require.Equal(t, regionID, *gotRegionID)
}

func TestValidateCreateOptInPreviewFeatureParameter(t *testing.T) {
	err := validateCreateOptInPreviewFeatureParameter(workspaceGroupResourceModel{
		OptInPreviewFeature: types.BoolValue(true),
		DeploymentType:      types.StringValue(string(management.WorkspaceGroupCreateDeploymentTypePRODUCTION)),
	})
	require.NotNil(t, err)
	require.Contains(t, err.Summary, "opt_in_preview_feature")

	require.Nil(t, validateCreateOptInPreviewFeatureParameter(workspaceGroupResourceModel{
		OptInPreviewFeature: types.BoolValue(true),
		DeploymentType:      types.StringValue(string(management.WorkspaceGroupCreateDeploymentTypeNONPRODUCTION)),
	}))
}

func TestValidateRequiredRegionParameters(t *testing.T) {
	err := validateRequiredRegionParameters(&workspaceGroupResourceModel{})
	require.NotNil(t, err)
	require.Contains(t, err.Summary, "Invalid region configuration")

	err = validateRequiredRegionParameters(&workspaceGroupResourceModel{
		RegionID:      types.StringValue(uuid.New().String()),
		CloudProvider: types.StringValue(string(management.CloudProviderAWS)),
		RegionName:    types.StringValue("us-east-1"),
	})
	require.NotNil(t, err)

	require.Nil(t, validateRequiredRegionParameters(&workspaceGroupResourceModel{
		CloudProvider: types.StringValue(string(management.CloudProviderAWS)),
		RegionName:    types.StringValue("us-east-1"),
	}))
}
