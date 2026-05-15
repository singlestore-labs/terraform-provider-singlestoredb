package flow

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/stretchr/testify/require"
)

type lookupResult struct {
	addrs []string
	err   error
}

type fakeHostResolver struct {
	results []lookupResult
	calls   int
}

func (r *fakeHostResolver) LookupHost(_ context.Context, _ string) ([]string, error) {
	result := r.results[r.calls]
	r.calls++

	return result.addrs, result.err
}

func TestWaitConditionEndpointReadyRequiresConsecutiveSuccessfulReadinessChecks(t *testing.T) {
	const endpoint = "example.com"

	lookupSuccess := lookupResult{addrs: []string{"93.184.216.34"}}
	lookupFailure := lookupResult{err: errors.New("temporary DNS failure")}
	results := make([]lookupResult, 0, config.FlowInstanceConsistencyThreshold*2)
	for range config.FlowInstanceConsistencyThreshold - 1 {
		results = append(results, lookupSuccess)
	}
	results = append(results, lookupFailure)
	for range config.FlowInstanceConsistencyThreshold {
		results = append(results, lookupSuccess)
	}

	resolver := &fakeHostResolver{results: results}
	condition := waitConditionEndpointReadyWithResolver(context.Background(), resolver)
	flow := management.Flow{
		FlowID:   uuid.MustParse("a1b2c3d4-5678-9abc-def0-123456789abc"),
		Endpoint: ptr(endpoint),
	}

	for range config.FlowInstanceConsistencyThreshold - 1 {
		err := condition(flow)
		require.ErrorContains(t, err, "readiness check did not pass consistently")
		require.NotContains(t, err.Error(), "test")
	}

	require.ErrorContains(t, condition(flow), "does not resolve in DNS yet")

	for range config.FlowInstanceConsistencyThreshold - 1 {
		require.ErrorContains(t, condition(flow), "readiness check did not pass consistently")
	}

	require.NoError(t, condition(flow))
	require.Equal(t, len(results), resolver.calls)
}

func ptr[T any](v T) *T {
	return &v
}
