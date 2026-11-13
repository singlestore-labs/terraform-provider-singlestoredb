package privateconnectionsv2_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"sync"
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

const privateConnectionV2Config = `
provider "singlestoredb" {
}

resource "singlestoredb_private_connection_v2" "this" {
  cluster_id = "283d4b0d-b0d6-485a-bc2d-a763c523c68a"
  allow_list = "651246146166"
  type       = "INBOUND"
}
`

func TestPrivateConnectionV2Resource(t *testing.T) {
	connectionID := uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f")
	clusterID := uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a")
	allowedList := "651246146166"
	updatedAllowList := "123456789012"
	status := management.ClusterPrivateConnectionStatusACTIVE
	connectionType := management.ClusterPrivateConnectionTypeINBOUND
	connection := management.ClusterPrivateConnection{
		ActiveAt:            util.Ptr("2025-01-21T11:11:38.145343Z"),
		AllowList:           &allowedList,
		CreatedAt:           util.Ptr("2025-01-21T11:11:38.145343Z"),
		UpdatedAt:           util.Ptr("2025-01-21T11:11:38.145343Z"),
		Endpoint:            util.Ptr("com.amazonaws.vpce.eu-central-1.vpce-svc-074a8eb58bb50c406"),
		OutboundAllowList:   util.Ptr("127.0.0.0"),
		PrivateConnectionID: connectionID,
		ClusterID:           &clusterID,
		ServiceName:         util.Ptr("test name"),
		Status:              &status,
		Type:                &connectionType,
		SqlPort:             util.Ptr(float32(3306)),
		WebsocketsPort:      util.Ptr(float32(443)),
	}

	mock := &privateConnectionV2Mock{
		t:                t,
		connection:       &connection,
		clusterID:        clusterID,
		connectionID:     connectionID,
		allowedList:      allowedList,
		updatedAllowList: updatedAllowList,
	}
	server := httptest.NewServer(mock)
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: privateConnectionV2Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", config.IDAttribute, connectionID.String()),
					resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "cluster_id", clusterID.String()),
					resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "allow_list", allowedList),
					resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "type", "INBOUND"),
					resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "status", "ACTIVE"),
					resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "service_name", "test name"),
					resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "endpoint", "com.amazonaws.vpce.eu-central-1.vpce-svc-074a8eb58bb50c406"),
					resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "sql_port", "3306"),
					resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "web_socket_port", "443"),
				),
			},
			{
				Config: testutil.UpdatableConfig(privateConnectionV2Config).
					WithPrivateConnectionV2Resource("this")("allow_list", cty.StringVal(updatedAllowList)).
					String(),
				Check: resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "allow_list", updatedAllowList),
			},
		},
	})

	require.GreaterOrEqual(t, mock.patches, 2)
	require.True(t, mock.deleted)
}

type privateConnectionV2Mock struct {
	t                *testing.T
	connection       *management.ClusterPrivateConnection
	clusterID        uuid.UUID
	connectionID     uuid.UUID
	allowedList      string
	updatedAllowList string
	mu               sync.Mutex
	gets             int
	patches          int
	deleted          bool
}

func (m *privateConnectionV2Mock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v2/privateConnections":
		m.create(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/v2/privateConnections/"+m.connectionID.String():
		m.get(w)
	case r.Method == http.MethodPatch && r.URL.Path == "/v2/privateConnections/"+m.connectionID.String():
		m.patch(w, r)
	case r.Method == http.MethodDelete && r.URL.Path == "/v2/privateConnections/"+m.connectionID.String():
		m.remove(w)
	default:
		m.t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (m *privateConnectionV2Mock) create(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	require.NoError(m.t, err)
	var input management.PrivateConnectionCreateV2
	require.NoError(m.t, json.Unmarshal(body, &input))
	require.Equal(m.t, m.clusterID, input.ClusterID)
	require.Equal(m.t, m.allowedList, util.Deref(input.AllowList))
	require.Nil(m.t, input.SqlPort)
	require.Nil(m.t, input.WebsocketsPort)
	writeConnectionID(m.t, w, m.connectionID)
}

func (m *privateConnectionV2Mock) get(w http.ResponseWriter) {
	m.gets++
	if m.deleted || m.gets == 1 {
		w.WriteHeader(http.StatusNotFound)

		return
	}
	_, err := w.Write(testutil.MustJSON(m.connection))
	require.NoError(m.t, err)
}

func (m *privateConnectionV2Mock) patch(w http.ResponseWriter, r *http.Request) {
	m.patches++
	if m.patches == 1 {
		w.WriteHeader(http.StatusInternalServerError)

		return
	}
	body, err := io.ReadAll(r.Body)
	require.NoError(m.t, err)
	var input management.PrivateConnectionUpdateV2
	require.NoError(m.t, json.Unmarshal(body, &input))
	require.Equal(m.t, m.updatedAllowList, util.Deref(input.AllowList))
	m.connection.AllowList = input.AllowList
	writeConnectionID(m.t, w, m.connectionID)
}

func (m *privateConnectionV2Mock) remove(w http.ResponseWriter) {
	m.deleted = true
	writeConnectionID(m.t, w, m.connectionID)
}

func writeConnectionID(t *testing.T, w http.ResponseWriter, id uuid.UUID) {
	t.Helper()
	_, err := w.Write(testutil.MustJSON(struct {
		PrivateConnectionID uuid.UUID `json:"privateConnectionID"` //nolint:tagliatelle // API uses privateConnectionID.
	}{PrivateConnectionID: id}))
	require.NoError(t, err)
}

func TestPrivateConnectionV2RejectsInboundWithoutAllowList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("API called for an invalid plan: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: `
provider "singlestoredb" {
}

resource "singlestoredb_private_connection_v2" "this" {
  cluster_id = "283d4b0d-b0d6-485a-bc2d-a763c523c68a"
  type       = "INBOUND"
}
`,
				ExpectError: regexp.MustCompile("allow_list configuration is required"),
			},
		},
	})
}

func TestPrivateConnectionV2ResourceIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.PrivateConnectionV2Resource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("singlestoredb_private_connection_v2.this", config.IDAttribute),
					resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "allow_list", "651246146166"),
					resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "type", "INBOUND"),
					resource.TestCheckResourceAttrSet("singlestoredb_private_connection_v2.this", "cluster_id"),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.PrivateConnectionV2Resource).
					WithPrivateConnectionV2Resource("this")("allow_list", cty.StringVal("123456789012")).
					String(),
				Check: resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "allow_list", "123456789012"),
			},
		},
	})
}

func TestPrivateConnectionV2ResourceKeepsConfiguredRegionsAndClusterID(t *testing.T) {
	connectionID := uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f")
	clusterID := uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a")
	mock := &regionsMock{t: t, connectionID: connectionID, clusterID: clusterID}
	server := httptest.NewServer(mock)
	t.Cleanup(server.Close)

	// The second region is unknown at plan time, and the cluster ID is uppercase.
	// The API returns the regions sorted and the cluster ID in lowercase.
	withRegions := func(regions string) string {
		return `
provider "singlestoredb" {
}

resource "terraform_data" "region" {
  input = "eu-west-1"
}

resource "singlestoredb_private_connection_v2" "this" {
  cluster_id              = "283D4B0D-B0D6-485A-BC2D-A763C523C68A"
  allow_list              = "651246146166"
  supported_cross_regions = ` + regions + `
}
`
	}

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: withRegions(`["us-west-2", terraform_data.region.output]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "cluster_id", "283D4B0D-B0D6-485A-BC2D-A763C523C68A"),
					resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "supported_cross_regions.0", "us-west-2"),
					resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "supported_cross_regions.1", "eu-west-1"),
				),
			},
			{
				Config: withRegions(`[]`),
				Check:  resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "supported_cross_regions.#", "0"),
			},
			{
				// A PENDING connection plans an update so the apply can wait for it.
				PreConfig:          func() { mock.setPending(1 << 30) },
				Config:             withRegions(`[]`),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				PreConfig: func() { mock.setPending(3) },
				Config:    withRegions(`[]`),
				Check:     resource.TestCheckResourceAttr("singlestoredb_private_connection_v2.this", "status", "ACTIVE"),
			},
		},
	})

	require.Equal(t, 1, mock.patches)
}

type regionsMock struct {
	t            *testing.T
	connectionID uuid.UUID
	clusterID    uuid.UUID
	mu           sync.Mutex
	regions      []string
	pendingGets  int
	patches      int
	deleted      bool
}

func (m *regionsMock) setPending(gets int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingGets = gets
}

func (m *regionsMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	path := "/v2/privateConnections/" + m.connectionID.String()
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v2/privateConnections":
		m.create(w, r)
	case r.Method == http.MethodGet && r.URL.Path == path:
		m.get(w)
	case r.Method == http.MethodPatch && r.URL.Path == path:
		m.patch(w, r)
	case r.Method == http.MethodDelete && r.URL.Path == path:
		m.deleted = true
		writeConnectionID(m.t, w, m.connectionID)
	default:
		m.t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (m *regionsMock) create(w http.ResponseWriter, r *http.Request) {
	var input management.PrivateConnectionCreateV2
	require.NoError(m.t, json.NewDecoder(r.Body).Decode(&input))
	require.Equal(m.t, m.clusterID, input.ClusterID)
	m.regions = append([]string(nil), util.Deref(input.SupportedCrossRegions)...)
	sort.Strings(m.regions)
	writeConnectionID(m.t, w, m.connectionID)
}

func (m *regionsMock) get(w http.ResponseWriter) {
	if m.deleted {
		w.WriteHeader(http.StatusNotFound)

		return
	}
	status := management.ClusterPrivateConnectionStatusACTIVE
	if m.pendingGets > 0 {
		m.pendingGets--
		status = management.ClusterPrivateConnectionStatusPENDING
	}
	regions := append([]string(nil), m.regions...)
	_, err := w.Write(testutil.MustJSON(management.ClusterPrivateConnection{
		PrivateConnectionID:   m.connectionID,
		ClusterID:             &m.clusterID,
		AllowList:             util.Ptr("651246146166"),
		ServiceName:           util.Ptr("service"),
		Status:                &status,
		Type:                  util.Ptr(management.ClusterPrivateConnectionTypeINBOUND),
		SupportedCrossRegions: &regions,
	}))
	require.NoError(m.t, err)
}

func (m *regionsMock) patch(w http.ResponseWriter, r *http.Request) {
	m.patches++
	var input management.PrivateConnectionUpdateV2
	require.NoError(m.t, json.NewDecoder(r.Body).Decode(&input))
	require.NotNil(m.t, input.SupportedCrossRegions)
	m.regions = append([]string(nil), *input.SupportedCrossRegions...)
	writeConnectionID(m.t, w, m.connectionID)
}
