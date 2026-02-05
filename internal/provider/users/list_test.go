package users_test

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
	"github.com/stretchr/testify/require"
)

func TestReadUsers(t *testing.T) {
	users := []management.User{
		{
			UserID:    uuid.MustParse("cf5b8cf7-7ee2-480d-97e1-dec270eb2df9"),
			Email:     "test1@user.com",
			FirstName: "Test1",
			LastName:  "User1",
		},
		{
			UserID:    uuid.MustParse("c9567f99-040a-4e42-8bf6-481e832e742a"),
			Email:     "test2@user.com",
			FirstName: "Test2",
			LastName:  "User2",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/users", r.URL.Path)
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(users))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.UserListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_users.all", "users.#", "2"),
					resource.TestCheckResourceAttr("data.singlestoredb_users.all", "users.0.id", users[0].UserID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_users.all", "users.0.email", users[0].Email),
					resource.TestCheckResourceAttr("data.singlestoredb_users.all", "users.0.first_name", users[0].FirstName),
					resource.TestCheckResourceAttr("data.singlestoredb_users.all", "users.0.last_name", users[0].LastName),
					resource.TestCheckResourceAttr("data.singlestoredb_users.all", "users.1.id", users[1].UserID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_users.all", "users.1.email", users[1].Email),
					resource.TestCheckResourceAttr("data.singlestoredb_users.all", "users.1.first_name", users[1].FirstName),
					resource.TestCheckResourceAttr("data.singlestoredb_users.all", "users.1.last_name", users[1].LastName),
				),
			},
		},
	})
}

func TestReadUsersError(t *testing.T) {
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
				Config:      examples.UserListDataSource,
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusUnauthorized)),
			},
		},
	})
}

func TestReadsUsersIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.UserListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_users.all", config.IDAttribute, config.TestIDValue),
					// Checking that at least no error.
				),
			},
		},
	})
}
