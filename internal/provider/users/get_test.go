package users_test

import (
	"fmt"
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
	"github.com/zclconf/go-cty/cty"
)

func TestReadsUser(t *testing.T) {
	user := management.User{
		UserID:    uuid.MustParse("a4df90a6-12b1-4de6-a50e-bd0a05aeaa09"),
		Email:     "test@user.com",
		FirstName: "Test",
		LastName:  "User",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, fmt.Sprintf("/v1beta/users/%s", user.UserID), r.URL.Path)
		w.Header().Add("Content-Type", "application/json") // Necessary to make the library parse the resulting JSON.
		_, err := w.Write(testutil.MustJSON(user))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.UserGetDataSource).
					WithUserGetDataSource("this")(config.IDAttribute, cty.StringVal(user.UserID.String())).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_user.this", config.IDAttribute, user.UserID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_user.this", "email", user.Email),
					resource.TestCheckResourceAttr("data.singlestoredb_user.this", "first_name", user.FirstName),
					resource.TestCheckResourceAttr("data.singlestoredb_user.this", "last_name", user.LastName),
				),
			},
		},
	})
}

func TestUserNotFound(t *testing.T) {
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
				Config: testutil.UpdatableConfig(examples.UserGetDataSource).
					WithUserGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)),
			},
		},
	})
}

func TestUserInvalidInputUUID(t *testing.T) {
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
				Config: testutil.UpdatableConfig(examples.UserGetDataSource).
					WithUserGetDataSource("this")(config.IDAttribute, cty.StringVal("valid-uuid")).
					String(),
				ExpectError: regexp.MustCompile("invalid UUID"),
			},
		},
	})
}

func TestGetUserNotFoundIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.UserGetDataSource).
					WithUserGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)), // Checking that at least the expected error.
			},
		},
	})
}
