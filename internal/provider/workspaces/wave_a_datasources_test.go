package workspaces_test

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

func TestReadsWorkspaceIdentity(t *testing.T) {
	workspaceID := uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")
	identity := management.CloudWorkloadIdentity{
		Identity: "arn:aws:iam::123456789012:role/s2-role",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, fmt.Sprintf("/v1/workspaces/%s/identity", workspaceID), r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(identity))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceIdentityDataSource).
					WithWorkspaceIdentityDataSource("this")("workspace_id", cty.StringVal(workspaceID.String())).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_identity.this", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_identity.this", "workspace_id", workspaceID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_identity.this", "identity", identity.Identity),
				),
			},
		},
	})
}

func TestReadsWorkspacePrivateConnections(t *testing.T) {
	workspaceID := uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")
	workspaceGroupID := uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")
	privateConnectionID := uuid.MustParse("a1a0a960-8591-4196-bb26-f53f0f8e35ce")
	sqlPort := float32(3306)
	webSocketPort := float32(443)
	privateConnections := []management.PrivateConnection{
		{
			PrivateConnectionID: privateConnectionID,
			WorkspaceGroupID:    workspaceGroupID,
			WorkspaceID:         util.Ptr(workspaceID),
			AllowList:           util.Ptr("123456789012"),
			OutboundAllowList:   util.Ptr("210987654321"),
			ServiceName:         util.Ptr("com.amazonaws.vpce.us-east-1.vpce-svc-123"),
			Endpoint:            util.Ptr("svc.singlestore.com"),
			Status:              util.Ptr(management.PrivateConnectionStatusACTIVE),
			Type:                util.Ptr(management.PrivateConnectionTypeINBOUND),
			SqlPort:             &sqlPort,
			WebsocketsPort:      &webSocketPort,
			AllowedPrivateLinkIDs: &[]string{
				"vpce-123",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, fmt.Sprintf("/v1/workspaces/%s/privateConnections", workspaceID), r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(privateConnections))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacePrivateConnections).
					WithWorkspacePrivateConnectionsDataSource("this")("workspace_id", cty.StringVal(workspaceID.String())).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_private_connections.this", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_private_connections.this", "workspace_id", workspaceID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_private_connections.this", "private_connections.#", "1"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_private_connections.this", "private_connections.0.id", privateConnectionID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_private_connections.this", "private_connections.0.workspace_group_id", workspaceGroupID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_private_connections.this", "private_connections.0.workspace_id", workspaceID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_private_connections.this", "private_connections.0.kai_endpoint_id", "vpce-123"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_private_connections.this", "private_connections.0.sql_port", "3306"),
				),
			},
		},
	})
}

func TestReadsWorkspacePrivateConnectionsKai(t *testing.T) {
	workspaceID := uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")
	response := management.PrivateConnectionKaiInfo{
		ServiceName: util.Ptr("com.amazonaws.vpce.us-east-1.vpce-svc-kai"),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, fmt.Sprintf("/v1/workspaces/%s/privateConnections/kai", workspaceID), r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(response))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacePrivateConnectionsKai).
					WithWorkspacePrivateConnectionsKaiDataSource("this")("workspace_id", cty.StringVal(workspaceID.String())).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_private_connections_kai.this", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_private_connections_kai.this", "workspace_id", workspaceID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_private_connections_kai.this", "service_name", *response.ServiceName),
				),
			},
		},
	})
}

func TestReadsWorkspacePrivateConnectionsOutboundAllowList(t *testing.T) {
	workspaceID := uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")
	response := []management.PrivateConnectionOutboundAllowList{
		{OutboundAllowList: util.Ptr("111111111111")},
		{OutboundAllowList: util.Ptr("222222222222")},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, fmt.Sprintf("/v1/workspaces/%s/privateConnections/outboundAllowList", workspaceID), r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(response))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacePrivateConnectionsOAL).
					WithWorkspacePrivateConnectionsOutboundAllowListDataSource("this")("workspace_id", cty.StringVal(workspaceID.String())).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_private_connections_outbound_allow_list.this", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_private_connections_outbound_allow_list.this", "workspace_id", workspaceID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_private_connections_outbound_allow_list.this", "outbound_allow_list.#", "2"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_private_connections_outbound_allow_list.this", "outbound_allow_list.0.outbound_allow_list", "111111111111"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_private_connections_outbound_allow_list.this", "outbound_allow_list.1.outbound_allow_list", "222222222222"),
				),
			},
		},
	})
}

func TestReadWorkspaceIdentityNotFoundIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceIdentityDataSource).
					WithWorkspaceIdentityDataSource("this")("workspace_id", cty.StringVal(uuid.New().String())).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound) + "|" + http.StatusText(http.StatusInternalServerError)),
			},
		},
	})
}
