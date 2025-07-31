package teams_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

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

func TestReadTeam(t *testing.T) {
	team := management.Team{
		TeamID:      uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"),
		Name:        "Test Team",
		Description: "This is a test team",
		MemberUsers: &[]management.UserInfo{
			{
				UserID:    uuid.MustParse("a4df90a6-12b1-4de6-a50e-bd0a05aeaa09"),
				Email:     "test@user.com",
				FirstName: "Test",
				LastName:  "User",
			},
		},
		MemberTeams: &[]management.TeamInfo{
			{
				TeamID:      uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a"),
				Name:        "Test Team 2",
				Description: "This is a test team 2",
			},
		},
		CreatedAt: util.Ptr("2025-01-21T11:11:38.145343Z"),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/teams", r.URL.Path)
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON([]management.Team{team}))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.TeamsGetDataSource).
					WithTeamGetDataSource("this")("name", cty.StringVal(team.Name)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_team.this", config.IDAttribute, team.TeamID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_team.this", "name", team.Name),
					resource.TestCheckResourceAttr("data.singlestoredb_team.this", "description", team.Description),
					resource.TestCheckResourceAttr("data.singlestoredb_team.this", "created_at", *team.CreatedAt),
					resource.TestCheckResourceAttr("data.singlestoredb_team.this", "member_users.#", "1"),
					resource.TestCheckResourceAttr("data.singlestoredb_team.this", "member_users.0.user_id", (*team.MemberUsers)[0].UserID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_team.this", "member_users.0.email", (*team.MemberUsers)[0].Email),
					resource.TestCheckResourceAttr("data.singlestoredb_team.this", "member_users.0.first_name", (*team.MemberUsers)[0].FirstName),
					resource.TestCheckResourceAttr("data.singlestoredb_team.this", "member_users.0.last_name", (*team.MemberUsers)[0].LastName),
					resource.TestCheckResourceAttr("data.singlestoredb_team.this", "member_teams.#", "1"),
					resource.TestCheckResourceAttr("data.singlestoredb_team.this", "member_teams.0.team_id", (*team.MemberTeams)[0].TeamID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_team.this", "member_teams.0.name", (*team.MemberTeams)[0].Name),
					resource.TestCheckResourceAttr("data.singlestoredb_team.this", "member_teams.0.description", (*team.MemberTeams)[0].Description),
				),
			},
		},
	})
}

func TestTeamNotFound(t *testing.T) {
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
				Config: testutil.UpdatableConfig(examples.TeamsGetDataSource).
					WithTeamGetDataSource("this")("name", cty.StringVal("foobar")).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)),
			},
		},
	})
}

func TestGetTeamNotFoundIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.TeamsGetDataSource).
					WithTeamGetDataSource("this")("name", cty.StringVal("foobarnosuchteam")).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)),
			},
		},
	})
}
