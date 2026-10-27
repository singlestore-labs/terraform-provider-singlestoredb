package util_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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

func TestAdminPasswordValidator(t *testing.T) {
	v := util.NewAdminPasswordValidator()
	validate := func(password string, skip types.Bool) validator.StringResponse {
		cfg := passwordConfig(t, types.StringValue(password), skip)
		var response validator.StringResponse
		v.ValidateString(t.Context(), validator.StringRequest{
			Path:        path.Root("admin_password"),
			ConfigValue: types.StringValue(password),
			Config:      cfg,
		}, &response)

		return response
	}

	require.Empty(t, validate(config.TestInitialAdminPassword, types.BoolValue(false)).Diagnostics)

	weak := validate("abcdefghijklmn", types.BoolValue(false))
	require.True(t, weak.Diagnostics.HasError())
	require.Contains(t, weak.Diagnostics.Errors()[0].Detail(), util.SkipAdminPasswordValidationAttribute+" = true")

	require.Empty(t, validate("abcdefghijklmn", types.BoolValue(true)).Diagnostics)
	require.Empty(t, validate("abcdefghijklmn", types.BoolUnknown()).Diagnostics)
	require.True(t, validate("abcdefghijklmn", types.BoolNull()).Diagnostics.HasError())
}

func passwordConfig(t *testing.T, password types.String, skip types.Bool) tfsdk.Config {
	t.Helper()

	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"admin_password": schema.StringAttribute{Optional: true},
		util.SkipAdminPasswordValidationAttribute: schema.BoolAttribute{Optional: true},
	}}
	pv, err := password.ToTerraformValue(t.Context())
	require.NoError(t, err)
	sv, err := skip.ToTerraformValue(t.Context())
	require.NoError(t, err)

	return tfsdk.Config{
		Schema: s,
		Raw: tftypes.NewValue(s.Type().TerraformType(t.Context()), map[string]tftypes.Value{
			"admin_password": pv,
			util.SkipAdminPasswordValidationAttribute: sv,
		}),
	}
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

	t.Run("deletes the cluster and explains the fix", func(t *testing.T) {
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
		require.Equal(t, "Admin password was rejected", got.Summary)
		require.Contains(t, got.Detail, "was deleted")
		require.Contains(t, got.Detail, util.AdminPasswordRules())
		require.Contains(t, got.Detail, "must include an uppercase letter")
		require.Contains(t, got.Detail, "Remove admin_password")
		require.NotContains(t, got.Detail, generated)
	})

	t.Run("points at changed API rules when the password passes the known rules", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"clusterID":"` + clusterID.String() + `"}`))
			require.NoError(t, err)
		}))
		t.Cleanup(server.Close)

		client, err := management.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		got := util.RejectReplacedAdminPassword(t.Context(), client, clusterID, config.TestInitialAdminPassword, generated)
		require.NotNil(t, got)
		require.Contains(t, got.Detail, "API rules have likely changed")
	})

	t.Run("reports a cluster that could not be deleted", func(t *testing.T) {
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
		require.Contains(t, got.Detail, "could not be deleted")
		require.Contains(t, got.Detail, clusterID.String())
		require.NotContains(t, got.Detail, generated)
	})
}
