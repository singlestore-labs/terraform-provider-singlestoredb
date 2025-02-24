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
					}),
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *workspaceGroupsDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	workspaceGroups, err := d.GetV1WorkspaceGroupsWithResponse(ctx, &management.GetV1WorkspaceGroupsParams{})
	if serr := util.StatusOK(workspaceGroups, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	result := workspaceGroupsListDataSourceModel{
		ID:              types.StringValue(config.TestIDValue),
		WorkspaceGroups: util.Map(util.Deref(workspaceGroups.JSON200), toWorkspaceGroupDataSourceModel),
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

func toWorkspaceGroupDataSourceModel(workspaceGroup management.WorkspaceGroup) workspaceGroupDataSourceModel {
	return workspaceGroupDataSourceModel{
		ID:                  util.UUIDStringValue(workspaceGroup.WorkspaceGroupID),
		Name:                types.StringValue(workspaceGroup.Name),
		State:               util.WorkspaceGroupStateStringValue(workspaceGroup.State),
		FirewallRanges:      util.FirewallRanges(workspaceGroup.FirewallRanges),
		AllowAllTraffic:     util.MaybeBoolValue(workspaceGroup.AllowAllTraffic),
		CreatedAt:           types.StringValue(workspaceGroup.CreatedAt),
		ExpiresAt:           util.MaybeStringValue(workspaceGroup.ExpiresAt),
		RegionID:            util.UUIDStringValue(workspaceGroup.RegionID),
		UpdateWindow:        toUpdateWindowDataSourceModel(workspaceGroup.UpdateWindow),
		DeploymentType:      util.StringValueOrNull(workspaceGroup.DeploymentType),
		OptInPreviewFeature: util.MaybeBoolValue(workspaceGroup.OptInPreviewFeature),
	}
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
