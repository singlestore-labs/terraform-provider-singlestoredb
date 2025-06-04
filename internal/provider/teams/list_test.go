package teams_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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
)

func TestReadTeams(t *testing.T) {
	teams := []management.Team{
		{
			TeamID:      uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"),
			Name:        "Test Team 1",
			Description: "This is a test team 1 ss",
			MemberUsers: &[]management.UserInfo{
				{
					UserID:    uuid.MustParse("a4df90a6-12b1-4de6-a50e-bd0a05aeaa09"),
					Email:     "test1@user.com",
					FirstName: "Test",
					LastName:  "User",
				},
			},
			MemberTeams: &[]management.TeamInfo{
				{
					TeamID:      uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a"),
					Name:        "Test Team inner",
					Description: "This is a test team inner",
				},
			},
			CreatedAt: util.Ptr("2025-01-21T11:11:38.145343Z"),
		},
		{
			TeamID:      uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a"),
			Name:        "Test Team 2",
			Description: "This is a test team 2",
			MemberUsers: &[]management.UserInfo{
				{
					UserID:    uuid.MustParse("a4df90a6-12b1-4de6-a50e-bd0a05aeaa09"),
					Email:     "test1@user1.com",
					FirstName: "Test1",
					LastName:  "User1",
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/teams", r.URL.Path)
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(teams))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.TeamsListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.#", "2"),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", fmt.Sprintf("teams.0.%s", config.IDAttribute), teams[0].TeamID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.0.name", teams[0].Name),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.0.description", teams[0].Description),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.0.created_at", *teams[0].CreatedAt),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.0.member_users.#", "1"),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.0.member_users.0.user_id", (*teams[0].MemberUsers)[0].UserID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.0.member_users.0.email", (*teams[0].MemberUsers)[0].Email),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.0.member_users.0.first_name", (*teams[0].MemberUsers)[0].FirstName),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.0.member_users.0.last_name", (*teams[0].MemberUsers)[0].LastName),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.0.member_teams.#", "1"),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.0.member_teams.0.team_id", (*teams[0].MemberTeams)[0].TeamID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.0.member_teams.0.name", (*teams[0].MemberTeams)[0].Name),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.0.member_teams.0.description", (*teams[0].MemberTeams)[0].Description),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", fmt.Sprintf("teams.1.%s", config.IDAttribute), teams[1].TeamID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.1.name", teams[1].Name),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.1.description", teams[1].Description),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.1.member_users.#", "1"),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.1.member_users.0.user_id", (*teams[1].MemberUsers)[0].UserID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.1.member_users.0.email", (*teams[1].MemberUsers)[0].Email),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.1.member_users.0.first_name", (*teams[1].MemberUsers)[0].FirstName),
					resource.TestCheckResourceAttr("data.singlestoredb_teams.all", "teams.1.member_users.0.last_name", (*teams[1].MemberUsers)[0].LastName),
				),
			},
		},
	})
}

func TestReadTeamsError(t *testing.T) {
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
				Config:      examples.TeamsListDataSource,
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusUnauthorized)),
			},
		},
	})
}
