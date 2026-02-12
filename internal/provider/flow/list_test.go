package flow_test

import (
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
)

func TestListsFlowInstances(t *testing.T) {
	flowInstance1 := management.Flow{
		FlowID:      uuid.MustParse("a1b2c3d4-5678-9abc-def0-123456789abc"),
		Name:        "flow-instance-1",
		WorkspaceID: util.Ptr(uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")),
		CreatedAt:   time.Date(2023, 2, 28, 5, 33, 6, 300300000, time.UTC),
		Endpoint:    util.Ptr("flow-svc-1.aws-oregon-3.svc.singlestore.com"),
		Size:        util.Ptr("F1"),
	}

	flowInstance2 := management.Flow{
		FlowID:      uuid.MustParse("b2c3d4e5-6789-0abc-1def-234567890abc"),
		Name:        "flow-instance-2",
		WorkspaceID: util.Ptr(uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")),
		CreatedAt:   time.Date(2023, 3, 15, 10, 20, 30, 400400000, time.UTC),
		Endpoint:    util.Ptr("flow-svc-2.aws-oregon-3.svc.singlestore.com"),
		Size:        util.Ptr("F2"),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/flow", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON([]management.Flow{flowInstance1, flowInstance2}))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowListDataSource).String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instances.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instances.all", "flow_instances.#", "2"),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instances.all", "flow_instances.0.id", flowInstance1.FlowID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instances.all", "flow_instances.0.name", flowInstance1.Name),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instances.all", "flow_instances.0.workspace_id", flowInstance1.WorkspaceID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instances.all", "flow_instances.0.endpoint", *flowInstance1.Endpoint),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instances.all", "flow_instances.0.size", *flowInstance1.Size),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instances.all", "flow_instances.1.id", flowInstance2.FlowID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instances.all", "flow_instances.1.name", flowInstance2.Name),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instances.all", "flow_instances.1.workspace_id", flowInstance2.WorkspaceID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instances.all", "flow_instances.1.endpoint", *flowInstance2.Endpoint),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instances.all", "flow_instances.1.size", *flowInstance2.Size),
				),
			},
		},
	})
}

func TestListFlowInstancesError(t *testing.T) {
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
				Config:      examples.FlowListDataSource,
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusUnauthorized)),
			},
		},
	})
}

func TestListsEmptyFlowInstances(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/flow", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON([]management.Flow{}))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowListDataSource).String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instances.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instances.all", "flow_instances.#", "0"),
				),
			},
		},
	})
}

func TestListEmptyFlowInstancesIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowListDataSource).String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instances.all", config.IDAttribute, config.TestIDValue),
				),
			},
		},
	})
}
