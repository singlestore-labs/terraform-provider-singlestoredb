package flow

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestToFlowInstanceResourceModel(t *testing.T) {
	t.Parallel()

	workspaceID := uuid.New()
	prior := &flowInstanceResourceModel{
		UserName:     types.StringValue("admin"),
		DatabaseName: types.StringValue("my_database"),
	}

	t.Run("uses API values when available", func(t *testing.T) {
		t.Parallel()

		flow := management.Flow{
			FlowID:       uuid.New(),
			Name:         "flow",
			WorkspaceID:  util.Ptr(workspaceID),
			CreatedAt:    time.Now().UTC(),
			Endpoint:     util.Ptr("new.example.com"),
			Size:         util.Ptr("F1"),
			UserName:     util.Ptr("api_user"),
			DatabaseName: util.Ptr("api_db"),
		}

		model := toFlowInstanceResourceModel(flow, prior)

		require.Equal(t, "new.example.com", model.Endpoint.ValueString())
		require.Equal(t, "api_user", model.UserName.ValueString())
		require.Equal(t, "api_db", model.DatabaseName.ValueString())
	})

	t.Run("preserves prior user fields when API returns placeholder", func(t *testing.T) {
		t.Parallel()

		flow := management.Flow{
			FlowID:       uuid.New(),
			Name:         "flow",
			WorkspaceID:  util.Ptr(workspaceID),
			CreatedAt:    time.Now().UTC(),
			Endpoint:     util.Ptr("example.com"),
			Size:         util.Ptr("F1"),
			UserName:     util.Ptr("Unknown"),
			DatabaseName: util.Ptr("unknown"),
		}

		model := toFlowInstanceResourceModel(flow, prior)

		require.Equal(t, "admin", model.UserName.ValueString())
		require.Equal(t, "my_database", model.DatabaseName.ValueString())
	})

	t.Run("leaves user fields unset without prior state", func(t *testing.T) {
		t.Parallel()

		flow := management.Flow{
			FlowID:       uuid.New(),
			Name:         "flow",
			WorkspaceID:  util.Ptr(workspaceID),
			CreatedAt:    time.Now().UTC(),
			UserName:     util.Ptr("Unknown"),
			DatabaseName: util.Ptr("Unknown"),
		}

		model := toFlowInstanceResourceModel(flow, nil)

		require.True(t, model.UserName.IsNull())
		require.True(t, model.DatabaseName.IsNull())
	})
}
