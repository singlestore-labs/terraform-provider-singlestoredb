package invitations_test

import (
	"fmt"
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
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestReadInvitations(t *testing.T) {
	invitations := []management.UserInvitation{
		{
			CreatedAt:    &createAt1,
			Email:        &email1,
			InvitationID: &id1,
			State:        &state1,
			TeamIDs:      &teams1,
		}, {
			CreatedAt:    &createAt2,
			InvitationID: &id2,
			Email:        &email2,
			State:        &state2,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/invitations", r.URL.Path)
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(invitations))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.InvitationsListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_invitations.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_invitations.all", "invitations.#", "2"),
					resource.TestCheckResourceAttr("data.singlestoredb_invitations.all", "invitations.0.id", invitations[0].InvitationID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_invitations.all", "invitations.0.created_at", util.MaybeTimeValue(invitations[0].CreatedAt).ValueString()),
					resource.TestCheckResourceAttr("data.singlestoredb_invitations.all", "invitations.0.state", string(*invitations[0].State)),
					resource.TestCheckResourceAttr("data.singlestoredb_invitations.all", "invitations.0.email", *invitations[0].Email),
					resource.TestCheckResourceAttr("data.singlestoredb_invitations.all", "invitations.0.teams.#", fmt.Sprintf("%d", len(*invitations[0].TeamIDs))),
					resource.TestCheckResourceAttr("data.singlestoredb_invitations.all", "invitations.0.teams.0", (*invitations[0].TeamIDs)[0].String()),
					resource.TestCheckResourceAttr("data.singlestoredb_invitations.all", "invitations.0.teams.1", (*invitations[0].TeamIDs)[1].String()),
					resource.TestCheckResourceAttr("data.singlestoredb_invitations.all", "invitations.1.id", invitations[1].InvitationID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_invitations.all", "invitations.1.created_at", util.MaybeTimeValue(invitations[1].CreatedAt).ValueString()),
					resource.TestCheckResourceAttr("data.singlestoredb_invitations.all", "invitations.1.state", string(*invitations[1].State)),
					resource.TestCheckResourceAttr("data.singlestoredb_invitations.all", "invitations.1.email", *invitations[1].Email),
					resource.TestCheckNoResourceAttr("data.singlestoredb_invitations.all", "invitations.1.teams.#"),
				),
			},
		},
	})
}

func TestReadInvitationsError(t *testing.T) {
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
				Config:      examples.InvitationsListDataSource,
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusUnauthorized)),
			},
		},
	})
}

func TestReadsInvitationsIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.InvitationsListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_invitations.all", config.IDAttribute, config.TestIDValue),
					// Checking that at least no error.
				),
			},
		},
	})
}
