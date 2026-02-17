package teams_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/examples"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

var team = management.Team{
	TeamID:      uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"),
	Name:        "terrafrom-test-team",
	Description: "Terrafrom test team",
}

var testUserMember = management.UserInfo{
	UserID:    uuid.MustParse("a4df90a6-12b1-4de6-a50e-bd0a05aeaa09"),
	Email:     "test@user.com",
	FirstName: "Test",
	LastName:  "User",
}

var testTeamMember = management.TeamInfo{
	TeamID:      uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a"),
	Name:        "Test Team 2",
	Description: "This is a test team 2",
}

var (
	nameUpdate        = "name updated"
	descriptionUpdate = "description updated"
)

func TestCRUDTeam(t *testing.T) {
	teamsPostHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/teams", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(
			struct {
				TeamID uuid.UUID
			}{
				TeamID: team.TeamID,
			},
		))
		require.NoError(t, err)
	}

	teamsGetHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != strings.Join([]string{"/v1/teams", team.TeamID.String()}, "/") ||
			r.Method != http.MethodGet {
			return false
		}

		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(team))
		require.NoError(t, err)

		return true
	}

	returnInternalError := true
	teamsPatchHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1/teams", team.TeamID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodPatch, r.Method)

		if returnInternalError {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)

			returnInternalError = false

			return
		}

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var input management.WorkspaceUpdate
		require.NoError(t, json.Unmarshal(body, &input))

		w.Header().Add("Content-Type", "application/json")
		_, err = w.Write(testutil.MustJSON(
			struct {
				TeamID uuid.UUID
			}{
				TeamID: team.TeamID,
			},
		))
		require.NoError(t, err)
		team.Name = nameUpdate
		team.Description = descriptionUpdate
		team.MemberUsers = &[]management.UserInfo{testUserMember}
		team.MemberTeams = &[]management.TeamInfo{testTeamMember}
	}

	teamsDeleteHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1/teams", team.TeamID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)

		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(
			struct {
				TeamID uuid.UUID
			}{
				TeamID: team.TeamID,
			},
		))
		require.NoError(t, err)
	}

	readOnlyHandlers := []func(w http.ResponseWriter, r *http.Request) bool{
		teamsGetHandler,
	}

	writeHandlers := []func(w http.ResponseWriter, r *http.Request){
		teamsPostHandler,
		teamsPatchHandler,
		teamsPatchHandler,
		teamsDeleteHandler,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range readOnlyHandlers {
			if h(w, r) {
				return
			}
		}

		h := writeHandlers[0]

		h(w, r)

		writeHandlers = writeHandlers[1:]
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.TeamsResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_team.this", config.IDAttribute, team.TeamID.String()),
					resource.TestCheckResourceAttr("singlestoredb_team.this", "name", team.Name),
					resource.TestCheckResourceAttr("singlestoredb_team.this", "description", team.Description),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.TeamsResource).
					WithTeamResource("this")("name", cty.StringVal(nameUpdate)).
					WithTeamResource("this")("description", cty.StringVal(descriptionUpdate)).
					WithTeamResource("this")("member_users", cty.ListVal([]cty.Value{cty.StringVal(testUserMember.Email)})).
					WithTeamResource("this")("member_teams", cty.ListVal([]cty.Value{cty.StringVal(testTeamMember.TeamID.String())})).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_team.this", config.IDAttribute, team.TeamID.String()),
					resource.TestCheckResourceAttr("singlestoredb_team.this", "name", nameUpdate),
					resource.TestCheckResourceAttr("singlestoredb_team.this", "description", descriptionUpdate),
					resource.TestCheckResourceAttr("singlestoredb_team.this", "member_teams.#", "1"),
					resource.TestCheckResourceAttr("singlestoredb_team.this", "member_users.#", "1"),
					resource.TestCheckResourceAttr("singlestoredb_team.this", "member_users.0", testUserMember.Email),
					resource.TestCheckResourceAttr("singlestoredb_team.this", "member_teams.0", testTeamMember.TeamID.String()),
				),
			},
		},
	})

	require.Empty(t, writeHandlers, "all the mutating REST calls should have been called, but %d is left not called yet", len(writeHandlers))
}

func TestTeamResourceIntegration(t *testing.T) {
	uniqueTeamName := testutil.GenerateUniqueResourceName("team")
	uniqueTeamNameUpdated := testutil.GenerateUniqueResourceName("team-updated")

	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.TeamsResource).
					WithTeamResource("this")("name", cty.StringVal(uniqueTeamName)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_team.this", "name", uniqueTeamName),
					resource.TestCheckResourceAttr("singlestoredb_team.this", "description", team.Description),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.TeamsResource).
					WithTeamResource("this")("name", cty.StringVal(uniqueTeamNameUpdated)).
					WithTeamResource("this")("description", cty.StringVal(descriptionUpdate)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_team.this", "name", uniqueTeamNameUpdated),
					resource.TestCheckResourceAttr("singlestoredb_team.this", "description", descriptionUpdate),
				),
			},
		},
	})
}
