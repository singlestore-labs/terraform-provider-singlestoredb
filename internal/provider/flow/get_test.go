package flow_test

import (
	"fmt"
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
	"github.com/zclconf/go-cty/cty"
)

var unset = cty.Value{}

func TestReadsFlowInstanceByID(t *testing.T) {
	flowInstance := management.Flow{
		FlowID:      uuid.MustParse("a1b2c3d4-5678-9abc-def0-123456789abc"),
		Name:        "test-flow-instance",
		WorkspaceID: util.Ptr(uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")),
		CreatedAt:   time.Date(2023, 2, 28, 5, 33, 6, 300300000, time.UTC),
		Endpoint:    util.Ptr("flow-svc-94a328d2-8c3d-412d.aws-oregon-3.svc.singlestore.com"),
		Size:        util.Ptr("F1"),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, fmt.Sprintf("/v1/flow/%s", flowInstance.FlowID), r.URL.Path)
		w.Header().Add("Content-Type", "json") // Necessary to make the library parse the resulting JSON.
		_, err := w.Write(testutil.MustJSON(flowInstance))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowGetDataSource).
					WithFlowInstanceGetDataSource("this")(config.IDAttribute, cty.StringVal(flowInstance.FlowID.String())).
					WithFlowInstanceGetDataSource("this")("name", unset).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instance.this", config.IDAttribute, flowInstance.FlowID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instance.this", "name", flowInstance.Name),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instance.this", "workspace_id", flowInstance.WorkspaceID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instance.this", "endpoint", *flowInstance.Endpoint),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instance.this", "size", *flowInstance.Size),
				),
			},
		},
	})
}

func TestFlowInstanceNotFoundByID(t *testing.T) {
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
				Config: testutil.UpdatableConfig(examples.FlowGetDataSource).
					WithFlowInstanceGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					WithFlowInstanceGetDataSource("this")("name", unset).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)),
			},
		},
	})
}

func TestFlowInstanceInvalidInputUUID(t *testing.T) {
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
				Config: testutil.UpdatableConfig(examples.FlowGetDataSource).
					WithFlowInstanceGetDataSource("this")(config.IDAttribute, cty.StringVal("invalid-uuid")).
					WithFlowInstanceGetDataSource("this")("name", unset).
					String(),
				ExpectError: regexp.MustCompile("invalid UUID"),
			},
		},
	})
}

func TestReadsFlowInstanceByName(t *testing.T) {
	flowInstance := management.Flow{
		FlowID:      uuid.MustParse("a1b2c3d4-5678-9abc-def0-123456789abc"),
		Name:        "my-flow-instance",
		WorkspaceID: util.Ptr(uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")),
		CreatedAt:   time.Date(2023, 2, 28, 5, 33, 6, 300300000, time.UTC),
		Endpoint:    util.Ptr("flow-svc-94a328d2-8c3d-412d.aws-oregon-3.svc.singlestore.com"),
		Size:        util.Ptr("F1"),
	}

	otherFlowInstance := management.Flow{
		FlowID:      uuid.MustParse("b2c3d4e5-6789-0abc-1def-234567890abc"),
		Name:        "other-flow-instance",
		WorkspaceID: util.Ptr(uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")),
		CreatedAt:   time.Date(2023, 2, 28, 5, 33, 6, 300300000, time.UTC),
		Endpoint:    util.Ptr("flow-svc-other.aws-oregon-3.svc.singlestore.com"),
		Size:        util.Ptr("F2"),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/flow", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON([]management.Flow{flowInstance, otherFlowInstance}))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowGetDataSource).
					WithFlowInstanceGetDataSource("this")(config.IDAttribute, unset).
					WithFlowInstanceGetDataSource("this")("name", cty.StringVal(flowInstance.Name)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instance.this", config.IDAttribute, flowInstance.FlowID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instance.this", "name", flowInstance.Name),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instance.this", "workspace_id", flowInstance.WorkspaceID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instance.this", "endpoint", *flowInstance.Endpoint),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instance.this", "size", *flowInstance.Size),
				),
			},
		},
	})
}

func TestFlowInstanceNotFoundByName(t *testing.T) {
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
				Config: testutil.UpdatableConfig(examples.FlowGetDataSource).
					WithFlowInstanceGetDataSource("this")(config.IDAttribute, unset).
					WithFlowInstanceGetDataSource("this")("name", cty.StringVal("nonexistent-flow")).
					String(),
				ExpectError: regexp.MustCompile("Flow instance not found"),
			},
		},
	})
}

func TestMultipleFlowInstancesFoundByName(t *testing.T) {
	flowInstance1 := management.Flow{
		FlowID:      uuid.MustParse("a1b2c3d4-5678-9abc-def0-123456789abc"),
		Name:        "duplicate-name",
		WorkspaceID: util.Ptr(uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")),
		CreatedAt:   time.Date(2023, 2, 28, 5, 33, 6, 300300000, time.UTC),
		Endpoint:    util.Ptr("flow-svc-1.aws-oregon-3.svc.singlestore.com"),
		Size:        util.Ptr("F1"),
	}

	flowInstance2 := management.Flow{
		FlowID:      uuid.MustParse("b2c3d4e5-6789-0abc-1def-234567890abc"),
		Name:        "duplicate-name",
		WorkspaceID: util.Ptr(uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")),
		CreatedAt:   time.Date(2023, 2, 28, 5, 33, 6, 300300000, time.UTC),
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
				Config: testutil.UpdatableConfig(examples.FlowGetDataSource).
					WithFlowInstanceGetDataSource("this")(config.IDAttribute, unset).
					WithFlowInstanceGetDataSource("this")("name", cty.StringVal("duplicate-name")).
					String(),
				ExpectError: regexp.MustCompile("Multiple Flow instances found"),
			},
		},
	})
}

func TestFlowInstanceTerminatedByID(t *testing.T) {
	terminatedAt := time.Date(2023, 3, 28, 5, 33, 6, 300300000, time.UTC)
	flowInstance := management.Flow{
		FlowID:      uuid.MustParse("a1b2c3d4-5678-9abc-def0-123456789abc"),
		Name:        "test-flow-instance",
		WorkspaceID: util.Ptr(uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")),
		CreatedAt:   time.Date(2023, 2, 28, 5, 33, 6, 300300000, time.UTC),
		DeletedAt:   &terminatedAt,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, fmt.Sprintf("/v1/flow/%s", flowInstance.FlowID), r.URL.Path)
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(flowInstance))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowGetDataSource).
					WithFlowInstanceGetDataSource("this")(config.IDAttribute, cty.StringVal(flowInstance.FlowID.String())).
					WithFlowInstanceGetDataSource("this")("name", unset).
					String(),
				ExpectError: regexp.MustCompile("terminated"),
			},
		},
	})
}

func TestFlowInstanceMissingIdentifier(t *testing.T) {
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
				Config: testutil.UpdatableConfig(examples.FlowGetDataSource).
					WithFlowInstanceGetDataSource("this")("name", unset).
					String(),
				ExpectError: regexp.MustCompile("Missing identifier"),
			},
		},
	})
}

func TestFlowInstanceConflictingIdentifiers(t *testing.T) {
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
				Config: testutil.UpdatableConfig(examples.FlowGetDataSource).
					WithFlowInstanceGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					WithFlowInstanceGetDataSource("this")("name", cty.StringVal("test-name")).
					String(),
				ExpectError: regexp.MustCompile("Conflicting identifiers"),
			},
		},
	})
}

func TestFlowInstanceByNameCaseInsensitive(t *testing.T) {
	flowInstance := management.Flow{
		FlowID:      uuid.MustParse("a1b2c3d4-5678-9abc-def0-123456789abc"),
		Name:        "My-Flow-Instance",
		WorkspaceID: util.Ptr(uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")),
		CreatedAt:   time.Date(2023, 2, 28, 5, 33, 6, 300300000, time.UTC),
		Endpoint:    util.Ptr("flow-svc-94a328d2-8c3d-412d.aws-oregon-3.svc.singlestore.com"),
		Size:        util.Ptr("F1"),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/flow", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON([]management.Flow{flowInstance}))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowGetDataSource).
					WithFlowInstanceGetDataSource("this")(config.IDAttribute, unset).
					WithFlowInstanceGetDataSource("this")("name", cty.StringVal("  my-flow-instance  ")).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instance.this", config.IDAttribute, flowInstance.FlowID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_flow_instance.this", "name", flowInstance.Name),
				),
			},
		},
	})
}

func TestGetFlowInstanceNotFoundByIDIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowGetDataSource).
					WithFlowInstanceGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					WithFlowInstanceGetDataSource("this")("name", unset).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)),
			},
		},
	})
}

func TestGetFlowInstanceNotFoundByNameIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowGetDataSource).
					WithFlowInstanceGetDataSource("this")(config.IDAttribute, unset).
					WithFlowInstanceGetDataSource("this")("name", cty.StringVal("non-existent-flow-instance-name-for-testing")).
					String(),
				ExpectError: regexp.MustCompile("Flow instance not found"),
			},
		},
	})
}
