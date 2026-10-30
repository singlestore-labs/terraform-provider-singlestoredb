package roles

import (
	"context"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// clusterCatalog is one operation's view of clusters and workspace groups.
// Grant, revoke, and the validation read that follows them share it, so a
// user_role create lists clusters once instead of once per lookup.
type clusterCatalog struct {
	client management.ClientWithResponsesInterface

	load           sync.Once
	loadErr        error
	clusterIDs     map[uuid.UUID]struct{}
	clusterToGroup map[uuid.UUID]uuid.UUID
	children       map[uuid.UUID][]uuid.UUID

	mu     sync.Mutex
	groups map[uuid.UUID]groupLookup
}

type groupLookup struct {
	exists bool
	err    error
}

type clusterCatalogKey struct{}

func newClusterCatalog(c management.ClientWithResponsesInterface) *clusterCatalog {
	return &clusterCatalog{
		client: c,
		groups: make(map[uuid.UUID]groupLookup),
	}
}

// withClusterCatalog attaches a catalog to ctx for the rest of one Terraform operation.
// A context that already has one is returned unchanged.
func withClusterCatalog(ctx context.Context, c management.ClientWithResponsesInterface) context.Context {
	if catalog, ok := ctx.Value(clusterCatalogKey{}).(*clusterCatalog); ok && catalog != nil {
		return ctx
	}

	return context.WithValue(ctx, clusterCatalogKey{}, newClusterCatalog(c))
}

func clusterCatalogFrom(ctx context.Context, c management.ClientWithResponsesInterface) *clusterCatalog {
	if catalog, ok := ctx.Value(clusterCatalogKey{}).(*clusterCatalog); ok && catalog != nil {
		return catalog
	}

	return newClusterCatalog(c)
}

func (cat *clusterCatalog) ensureLoaded(ctx context.Context) error {
	cat.load.Do(func() {
		list, err := cat.client.GetV2ClustersWithResponse(ctx, &management.GetV2ClustersParams{})
		if serr := util.StatusOK(list, err); serr != nil {
			cat.loadErr = serr

			return
		}

		clusters := util.Deref(list.JSON200)
		cat.clusterIDs = make(map[uuid.UUID]struct{}, len(clusters))
		cat.clusterToGroup = make(map[uuid.UUID]uuid.UUID, len(clusters))
		cat.children = make(map[uuid.UUID][]uuid.UUID)
		for _, cluster := range clusters {
			if cluster.ClusterID == nil {
				continue
			}
			id := *cluster.ClusterID
			cat.clusterIDs[id] = struct{}{}
			if cluster.GroupID == nil {
				continue
			}
			cat.clusterToGroup[id] = *cluster.GroupID
			cat.children[*cluster.GroupID] = append(cat.children[*cluster.GroupID], id)
		}
	})

	return cat.loadErr
}

func (cat *clusterCatalog) isCluster(id uuid.UUID) bool {
	_, ok := cat.clusterIDs[id]

	return ok
}

// workspaceGroupExists reports false only on 404. Other failures are returned so
// they are not mistaken for a missing workspace group. Repeated lookups of the
// same ID in one catalog reuse the first response.
func (cat *clusterCatalog) workspaceGroupExists(ctx context.Context, resourceID uuid.UUID) (bool, error) {
	cat.mu.Lock()
	defer cat.mu.Unlock()

	if cached, ok := cat.groups[resourceID]; ok {
		return cached.exists, cached.err
	}

	exists, err := fetchWorkspaceGroupExists(ctx, cat.client, resourceID)
	cat.groups[resourceID] = groupLookup{exists: exists, err: err}

	return exists, err
}

func fetchWorkspaceGroupExists(ctx context.Context, c management.ClientWithResponsesInterface, resourceID uuid.UUID) (bool, error) {
	group, err := c.GetV1WorkspaceGroupsWorkspaceGroupIDWithResponse(ctx, resourceID, &management.GetV1WorkspaceGroupsWorkspaceGroupIDParams{})
	if serr := util.StatusOK(group, err, util.ReturnNilOnNotFound); serr != nil {
		return false, serr
	}

	return group.StatusCode() == http.StatusOK && group.JSON200 != nil, nil
}
