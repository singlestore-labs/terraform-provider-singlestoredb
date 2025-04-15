package users_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/examples"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/stretchr/testify/require"
)

func TestCreateDeleteUser(t *testing.T) {
	userID := uuid.MustParse("a4df90a6-12b1-4de6-a50e-bd0a05aeaa09")

	user := management.User{
		UserID:    userID,
		Email:     "test@user.com",
		FirstName: "Test",
		LastName:  "User",
	}

	usersGetByEmailHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/v1beta/users" || r.URL.RawQuery != "email=test%40user.com" || r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON([]management.User{user}))
		require.NoError(t, err)

		return true
	}

	usersGetByIDHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != strings.Join([]string{"/v1beta/users", userID.String()}, "/") || r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(user))
		require.NoError(t, err)

		return true
	}

	usersPostHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1beta/users", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var input management.PostV1betaUsersJSONBody
		require.NoError(t, json.Unmarshal(body, &input))
		require.Equal(t, user.Email, string(input.Email))

		w.Header().Add("Content-Type", "application/json")
		require.NoError(t, err)
	}

	usersDeleteHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1beta/users", userID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)
	}

	readOnlyHandlers := []func(w http.ResponseWriter, r *http.Request) bool{
		usersGetByEmailHandler,
		usersGetByIDHandler,
	}

	writeHandlers := []func(w http.ResponseWriter, r *http.Request){
		usersPostHandler,
		usersDeleteHandler,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range readOnlyHandlers {
			if h(w, r) {
				return
			}
		}

		require.NotEmpty(t, writeHandlers, "already executed all the expected mutating REST calls")

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
				Config: examples.UserResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_user.this", config.IDAttribute, userID.String()),
					resource.TestCheckResourceAttr("singlestoredb_user.this", "email", user.Email),
					resource.TestCheckResourceAttr("singlestoredb_user.this", "first_name", user.FirstName),
					resource.TestCheckResourceAttr("singlestoredb_user.this", "last_name", user.LastName),
				),
			},
		},
	})

	require.Empty(t, writeHandlers, "all the mutating REST calls should have been called, but %d is left not called yet", len(writeHandlers))
}
