package privateconnections_test

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
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestReadsPrivateConnection(t *testing.T) {
	privateConnection := management.PrivateConnection{
		ActiveAt:            util.Ptr("2025-01-21T11:11:38.145343Z"),
		AllowList:           util.Ptr("12345"),
		CreatedAt:           util.Ptr("2025-01-21T11:11:38.145343Z"),
		UpdatedAt:           util.Ptr("2025-01-21T11:11:38.145343Z"),
		Endpoint:            util.Ptr("com.amazonaws.vpce.eu-central-1.vpce-svc-074a8eb58bb50c406"),
		OutboundAllowList:   util.Ptr("127.0.0.0"),
		PrivateConnectionID: uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"),
		ServiceName:         util.Ptr("test name"),
		Status:              util.Ptr(management.PrivateConnectionStatusACTIVE),
		Type:                util.Ptr(management.PrivateConnectionTypeINBOUND),
		WorkspaceID:         util.Ptr(uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a")),
		WorkspaceGroupID:    uuid.MustParse("a4df90a6-e2b2-4de6-a50e-bd0a05aeaa09"),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, fmt.Sprintf("/v1/privateConnections/%s", privateConnection.PrivateConnectionID), r.URL.Path)
		w.Header().Add("Content-Type", "json") // Necessary to make the library parse the resulting JSON.
		_, err := w.Write(testutil.MustJSON(privateConnection))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.PrivateConnectionsGetDataSource).
					WithPrivateConnectionGetDataSource("this")(config.IDAttribute, cty.StringVal(privateConnection.PrivateConnectionID.String())).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection.this", config.IDAttribute, privateConnection.PrivateConnectionID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection.this", "active_at", "2025-01-21T11:11:38.145343Z"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection.this", "allow_list", "12345"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection.this", "created_at", "2025-01-21T11:11:38.145343Z"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection.this", "endpoint", "com.amazonaws.vpce.eu-central-1.vpce-svc-074a8eb58bb50c406"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection.this", "outbound_allow_list", "127.0.0.0"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection.this", "service_name", "test name"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection.this", "status", "ACTIVE"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection.this", "type", "INBOUND"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection.this", "workspace_id", "283d4b0d-b0d6-485a-bc2d-a763c523c68a"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection.this", "workspace_group_id", "a4df90a6-e2b2-4de6-a50e-bd0a05aeaa09"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection.this", "updated_at", "2025-01-21T11:11:38.145343Z"),
				),
			},
		},
	})
}

func TestPrivateConnectionNotFound(t *testing.T) {
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
				Config: testutil.UpdatableConfig(examples.PrivateConnectionsGetDataSource).
					WithPrivateConnectionGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)),
			},
		},
	})
}

func TestPrivateConnectionInvalidInputUUID(t *testing.T) {
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
				Config: testutil.UpdatableConfig(examples.PrivateConnectionsGetDataSource).
					WithPrivateConnectionGetDataSource("this")(config.IDAttribute, cty.StringVal("valid-uuid")).
					String(),
				ExpectError: regexp.MustCompile("invalid UUID"),
			},
		},
	})
}

func TestGetPrivateConnectionNotFoundIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.PrivateConnectionsGetDataSource).
					WithPrivateConnectionGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)), // Checking that at least the expected error.
			},
		},
	})
}
