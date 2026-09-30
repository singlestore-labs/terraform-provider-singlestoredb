package workspacegroups

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	dataSourceListName = "workspace_groups"
)

// workspaceGroupsDataSourceList is the data source implementation.
type workspaceGroupsDataSourceList struct {
	management.ClientWithResponsesInterface
}

// workspaceGroupsListDataSourceModel maps the data source schema data.
type workspaceGroupsListDataSourceModel struct {
	ID              types.String                    `tfsdk:"id"`
	WorkspaceGroups []workspaceGroupDataSourceModel `tfsdk:"workspace_groups"`
}

type updateWindowDataSourceModel struct {
	Hour types.Int64 `tfsdk:"hour"`
	Day  types.Int64 `tfsdk:"day"`
}

var _ datasource.DataSourceWithConfigure = &workspaceGroupsDataSourceList{}

// NewDataSourceList is a helper function to simplify the provider implementation.
func NewDataSourceList() datasource.DataSource {
	return &workspaceGroupsDataSourceList{}
}

// Metadata returns the data source type name.
func (d *workspaceGroupsDataSourceList) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, dataSourceListName)
}

// Schema defines the schema for the data source.
func (d *workspaceGroupsDataSourceList) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "This data source provides a list of workspace groups that the user has access to.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed: true,
			},
			dataSourceListName: schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: newWorkspaceGroupDataSourceSchemaAttributes(workspaceGroupDataSourceSchemaConfig{
						computeWorkspaceGroupID: true,
						computeName:             true,
					}),
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *workspaceGroupsDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	clusters, serr := listClusters(ctx, d.ClientWithResponsesInterface)
	if serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	groups := uniqueGroupsByGroupID(clusters)
	result := workspaceGroupsListDataSourceModel{
		ID: types.StringValue(config.TestIDValue),
		WorkspaceGroups: util.Map(groups, func(c management.Cluster) workspaceGroupDataSourceModel {
			return toWorkspaceGroupDataSourceModel(ctx, d.ClientWithResponsesInterface, c)
		}),
	}

	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *workspaceGroupsDataSourceList) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func toWorkspaceGroupDataSourceModel(ctx context.Context, c management.ClientWithResponsesInterface, workspaceGroup management.Cluster) workspaceGroupDataSourceModel {
	model := workspaceGroupDataSourceModel{
		ID:                       util.MaybeUUIDStringValue(workspaceGroup.GroupID),
		Name:                     types.StringValue(workspaceGroup.Name),
		ProjectName:              resolveProjectName(ctx, c, workspaceGroup.ProjectID),
		State:                    util.ClusterStateStringValue(clusterState(workspaceGroup)),
		FirewallRanges:           util.FirewallRanges(util.Ptr(effectiveFirewallRanges(workspaceGroup))),
		AllowAllTraffic:          util.MaybeBoolValue(workspaceGroup.AllowAllTraffic),
		CreatedAt:                clusterCreatedAtString(workspaceGroup),
		ExpiresAt:                util.MaybeStringValue(workspaceGroup.ExpiresAt),
		RegionID:                 types.StringNull(),
		UpdateWindow:             toUpdateWindowDataSourceModel(workspaceGroup.UpdateWindow),
		DeploymentType:           util.StringValueOrNull(workspaceGroup.DeploymentType),
		OptInPreviewFeature:      util.MaybeBoolValue(workspaceGroup.OptInPreviewFeature),
		HighAvailabilityTwoZones: util.MaybeBoolValue(workspaceGroup.MultiAZ),
		OutboundAllowList:        util.MaybeStringValue(workspaceGroup.OutboundAllowList),
	}
	if workspaceGroup.Provider != nil {
		model.CloudProvider = types.StringValue(string(*workspaceGroup.Provider))
	}
	model.RegionName = util.MaybeStringValue(workspaceGroup.Region)

	return model
}

func toUpdateWindowDataSourceModel(uw *management.UpdateWindow) *updateWindowDataSourceModel {
	if uw == nil {
		return nil
	}

	return &updateWindowDataSourceModel{
		Hour: types.Int64Value(int64(uw.Hour)),
		Day:  types.Int64Value(int64(uw.Day)),
	}
}
