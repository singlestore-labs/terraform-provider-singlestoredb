package users_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
)

var (
	createAt1 = time.Now().Local()
	id1       = uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f")
	userID    = uuid.MustParse("a4df90a6-12b1-4de6-a50e-bd0a05aeaa09")
	email1    = "test@user.com"
	state1    = management.Pending
)

var invitation = management.UserInvitation{
	CreatedAt:    &createAt1,
	InvitationID: &id1,
	Email:        &email1,
	State:        &state1,
}

var user = management.User{
	UserID:    userID,
	Email:     email1,
	FirstName: "Test",
	LastName:  "User",
}

func setupHandlers(t *testing.T, acceptInvitation bool) ([]func(w http.ResponseWriter, r *http.Request) bool, []func(w http.ResponseWriter, r *http.Request)) {
	t.Helper()
	invitationsGetHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != strings.Join([]string{"/v1beta/invitations", invitation.InvitationID.String()}, "/") ||
			r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(invitation))
		require.NoError(t, err)

		return true
	}

	usersGetHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/v1beta/users" || r.URL.RawQuery != "email=test%40user.com" ||
			r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "application/json")
		if acceptInvitation {
			// Simulate that the user has accepted the invitation
			_, err := w.Write(testutil.MustJSON([]management.User{user}))
			require.NoError(t, err)
		} else {
			_, err := w.Write(testutil.MustJSON([]management.User{}))
			require.NoError(t, err)
		}

		return true
	}

	invitationsPostHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1beta/invitations", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var input management.UserInvitationCreate
		require.NoError(t, json.Unmarshal(body, &input))
		require.Equal(t, *invitation.Email, string(input.Email))

		w.Header().Add("Content-Type", "application/json")
		_, err = w.Write(testutil.MustJSON(invitation))
		require.NoError(t, err)
	}

	invitationRevokeHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1beta/invitations", invitation.InvitationID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)

		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(
			struct {
				InvitationID uuid.UUID
			}{
				InvitationID: id1,
			},
		))
		require.NoError(t, err)
	}

	userRemoveHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1beta/users", user.UserID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)

		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(
			struct {
				UserID uuid.UUID
			}{
				UserID: user.UserID,
			},
		))
		require.NoError(t, err)
	}

	readOnlyHandlers := []func(w http.ResponseWriter, r *http.Request) bool{
		invitationsGetHandler,
		usersGetHandler,
	}

	writeHandlers := []func(w http.ResponseWriter, r *http.Request){
		invitationsPostHandler,
	}

	if acceptInvitation {
		writeHandlers = append(writeHandlers, userRemoveHandler)
	} else {
		writeHandlers = append(writeHandlers, invitationRevokeHandler)
	}

	return readOnlyHandlers, writeHandlers
}

func runInvitationsTest(t *testing.T, acceptInvitation bool) {
	t.Helper()
	readOnlyHandlers, writeHandlers := setupHandlers(t, acceptInvitation)

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
					resource.TestCheckResourceAttr("singlestoredb_user.this", config.IDAttribute, invitation.InvitationID.String()),
					resource.TestCheckResourceAttr("singlestoredb_user.this", "created_at", util.MaybeTimeValue(invitation.CreatedAt).ValueString()),
					resource.TestCheckResourceAttr("singlestoredb_user.this", "state", string(*invitation.State)),
					resource.TestCheckResourceAttr("singlestoredb_user.this", "email", *invitation.Email),
				),
			},
		},
	})

	require.Empty(t, writeHandlers, "all the mutating REST calls should have been called, but %d is left not called yet", len(writeHandlers))
}

func TestInviteAndRevokeInvitation(t *testing.T) {
	runInvitationsTest(t, false)
}

func TestInviteAcceptAndRemoveUser(t *testing.T) {
	runInvitationsTest(t, true)
}

func TestInvitationResourceIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.UserResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_user.this", "state", string(*invitation.State)),
					resource.TestCheckResourceAttr("singlestoredb_user.this", "email", *invitation.Email),
				),
			},
		},
	})
}
