package privateconnections_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/privateconnections"
	"github.com/stretchr/testify/require"
)

func TestWaitConditionAllowListNilAllowList(t *testing.T) {
	condition := privateconnections.WaitConditionAllowList("301668617982")

	var err error
	require.NotPanics(t, func() {
		err = condition(management.PrivateConnection{
			PrivateConnectionID: uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"),
			AllowList:           nil,
		})
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "should be 301668617982")
}

func TestWaitConditionAllowListMismatch(t *testing.T) {
	condition := privateconnections.WaitConditionAllowList("301668617982")

	current := "123456789012"
	err := condition(management.PrivateConnection{
		PrivateConnectionID: uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"),
		AllowList:           &current,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), current)
}

func TestWaitConditionAllowListMatch(t *testing.T) {
	desired := "301668617982"
	condition := privateconnections.WaitConditionAllowList(desired)

	require.NoError(t, condition(management.PrivateConnection{
		PrivateConnectionID: uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"),
		AllowList:           &desired,
	}))
}
