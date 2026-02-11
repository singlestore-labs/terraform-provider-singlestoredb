package invitations_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/examples"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

var (
	createAt1 = time.Now().Local()
	id1       = uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f")
	email1    = "user1@user.com"
	state1    = management.Pending
	teams1    = []uuid.UUID{
		uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a"),
		uuid.MustParse("a4df90a6-e2b2-4de6-a50e-bd0a05aeaa09"),
	}
	createAt2 = time.Now().Local().Add(-1 * time.Hour)
	id2       = uuid.MustParse("a4df90a6-e2b2-4de6-a50e-bd0a05aeaa09")
	email2    = "user2@user.com"
	state2    = management.Pending
)

func TestReadsInvitation(t *testing.T) {
	invitation := management.UserInvitation{
		CreatedAt:    &createAt1,
		InvitationID: &id1,
		Email:        &email1,
		State:        &state1,
		TeamIDs:      &teams1,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, fmt.Sprintf("/v1/invitations/%s", invitation.InvitationID), r.URL.Path)
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(invitation))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.InvitationsGetDataSource).
					WithInvitationGetDataSource("this")(config.IDAttribute, cty.StringVal(invitation.InvitationID.String())).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_invitation.this", config.IDAttribute, invitation.InvitationID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_invitation.this", "created_at", util.MaybeTimeValue(invitation.CreatedAt).ValueString()),
					resource.TestCheckResourceAttr("data.singlestoredb_invitation.this", "state", string(*invitation.State)),
					resource.TestCheckResourceAttr("data.singlestoredb_invitation.this", "email", *invitation.Email),
					resource.TestCheckResourceAttr("data.singlestoredb_invitation.this", "teams.#", fmt.Sprintf("%d", len(*invitation.TeamIDs))),
					resource.TestCheckResourceAttr("data.singlestoredb_invitation.this", "teams.0", (*invitation.TeamIDs)[0].String()),
					resource.TestCheckResourceAttr("data.singlestoredb_invitation.this", "teams.1", (*invitation.TeamIDs)[1].String()),
				),
			},
		},
	})
}

func TestInvitationNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        "bar",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.InvitationsGetDataSource).
					WithInvitationGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)),
			},
		},
	})
}

func TestInvitationInvalidInputUUID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.False(t, true, "should not get here")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        "bar",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.InvitationsGetDataSource).
					WithInvitationGetDataSource("this")(config.IDAttribute, cty.StringVal("valid-uuid")).
					String(),
				ExpectError: regexp.MustCompile("invalid UUID"),
			},
		},
	})
}

func TestGetInvitationNotFoundIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.InvitationsGetDataSource).
					WithInvitationGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)), // Checking that at least the expected error.
			},
		},
	})
}
