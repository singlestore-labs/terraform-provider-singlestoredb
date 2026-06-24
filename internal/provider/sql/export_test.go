package sql

import (
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// ResolvePasswordForTest exposes resolvePassword for external tests.
func ResolvePasswordForTest(attr types.String) (string, *util.SummaryWithDetailError) {
	return resolvePassword(attr)
}

// PasswordForStateForTest exposes passwordForState for external tests.
func PasswordForStateForTest(attr types.String) types.String {
	return passwordForState(attr)
}

// PasswordConfiguredInPlanForTest exposes passwordConfiguredInPlan for external tests.
func PasswordConfiguredInPlanForTest(attr types.String) bool {
	return passwordConfiguredInPlan(attr)
}

// QueryDataSourceIDForTest exposes queryDataSourceID for external tests.
func QueryDataSourceIDForTest(endpoint, query string, args []string) string {
	return queryDataSourceID(endpoint, query, args)
}

// FirstResultSetRowsForTest exposes firstResultSetRows for external tests.
func FirstResultSetRowsForTest(resp *QueryRowsResponse) []map[string]any {
	return firstResultSetRows(resp)
}

// SetHTTPClientFactoryForTest overrides the HTTP client used by NewClient. Returns a restore func.
func SetHTTPClientFactoryForTest(factory func() *http.Client) func() {
	prev := httpClientFactory
	httpClientFactory = factory

	return func() {
		httpClientFactory = prev
	}
}
