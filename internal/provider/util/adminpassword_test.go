package util_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestAdminPasswordForState(t *testing.T) {
	require.Equal(t, "configured", util.AdminPasswordForState("configured", "generated"))
	require.Equal(t, "configured", util.AdminPasswordForState("configured", ""))
	require.Equal(t, "generated", util.AdminPasswordForState("", "generated"))
}

func TestAdminPasswordProblems(t *testing.T) {
	require.Empty(t, util.AdminPasswordProblems(config.TestInitialAdminPassword))
	require.Empty(t, util.AdminPasswordProblems("mockPasswordUpdated193!"))
	require.Empty(t, util.AdminPasswordProblems("HasaaaRepeat19!"))
	require.Empty(t, util.AdminPasswordProblems("Alt121Rest!AbXy"))

	require.NotEmpty(t, util.AdminPasswordProblems("short"))
	require.NotEmpty(t, util.AdminPasswordProblems("alllowercase19!"))
	require.NotEmpty(t, util.AdminPasswordProblems("ALLUPPERCASE19!"))
	require.NotEmpty(t, util.AdminPasswordProblems("NoDigitPassword!"))
	require.NotEmpty(t, util.AdminPasswordProblems("NoSpecialChar1945"))
	require.NotEmpty(t, util.AdminPasswordProblems("HasSeqabc1945!X"))
	require.NotEmpty(t, util.AdminPasswordProblems("RevcbaRest19!Xy"))
	require.NotEmpty(t, util.AdminPasswordProblems("RepeatAAAA19!xyZ"))
}

func TestAdminPasswordWasReplaced(t *testing.T) {
	require.False(t, util.AdminPasswordWasReplaced("", "generated"))
	require.False(t, util.AdminPasswordWasReplaced("configured", ""))
	require.False(t, util.AdminPasswordWasReplaced("same", "same"))
	require.True(t, util.AdminPasswordWasReplaced("configured", "generated"))
}

func TestRejectReplacedAdminPassword(t *testing.T) {
	require.Nil(t, util.RejectReplacedAdminPassword(t.Context(), nil, uuid.Nil, "", "generated"))
	require.Nil(t, util.RejectReplacedAdminPassword(t.Context(), nil, uuid.Nil, "configured", ""))
	require.Nil(t, util.RejectReplacedAdminPassword(t.Context(), nil, uuid.Nil, "same", "same"))

	clusterID := uuid.MustParse(config.TestInitialWorkspaceID)
	const generated = "generated-secret-value"

	t.Run("deletes the cluster", func(t *testing.T) {
		var deleted bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodDelete, r.Method)
			require.Contains(t, r.URL.Path, clusterID.String())
			deleted = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"clusterID":"` + clusterID.String() + `"}`))
			require.NoError(t, err)
		}))
		t.Cleanup(server.Close)

		client, err := management.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		got := util.RejectReplacedAdminPassword(t.Context(), client, clusterID, "configured-secret", generated)
		require.NotNil(t, got)
		require.True(t, deleted)
		require.NotContains(t, got.Detail, generated)
	})

	t.Run("keeps the generated password when delete fails", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte(`{"message":"nope"}`))
			require.NoError(t, err)
		}))
		t.Cleanup(server.Close)

		client, err := management.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		got := util.RejectReplacedAdminPassword(t.Context(), client, clusterID, "configured-secret", generated)
		require.NotNil(t, got)
		require.Contains(t, got.Detail, generated)
		require.Contains(t, got.Detail, clusterID.String())
	})
}
