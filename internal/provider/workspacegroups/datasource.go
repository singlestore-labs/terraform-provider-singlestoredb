package workspacegroups

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/util"
)

const (
	dataSourceName = "workspace_groups"
)

// workspaceGroupsDataSource is the data source implementation.
type workspaceGroupsDataSource struct {
	management.ClientWithResponsesInterface
}

// workspaceGroupsDataSourceModel maps the data source schema data.
type workspaceGroupsDataSourceModel struct {
	WorkspaceGroups []workspaceGroupDataSourceModel `tfsdk:"workspace_groups"`
	ID              types.String                    `tfsdk:"id"`
}

// workspaceGroupDataSourceModel maps workspace groups schema data.
type workspaceGroupDataSourceModel struct {
	Name             types.String                 `tfsdk:"name"`
	State            types.String                 `tfsdk:"state"`
	WorkspaceGroupID types.String                 `tfsdk:"workspace_group_id"`
	FirewallRanges   []types.String               `tfsdk:"firewall_ranges"`
	AllowAllTraffic  types.Bool                   `tfsdk:"allow_all_traffic"`
	CreatedAt        types.String                 `tfsdk:"created_at"`
	ExpiresAt        types.String                 `tfsdk:"expires_at"`
	TerminatedAt     types.String                 `tfsdk:"terminated_at"`
	RegionID         types.String                 `tfsdk:"region_id"`
	UpdateWindow     *updateWindowDataSourceModel `tfsdk:"update_window"`
}

type updateWindowDataSourceModel struct {
	Hour types.Int64 `tfsdk:"hour"`
	Day  types.Int64 `tfsdk:"day"`
}

var _ datasource.DataSourceWithConfigure = &workspaceGroupsDataSource{}

// NewDataSource is a helper function to simplify the provider implementation.
func NewDataSource() datasource.DataSource {
	return &workspaceGroupsDataSource{}
}

// Metadata returns the data source type name.
func (d *workspaceGroupsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, dataSourceName)
}

// Schema defines the schema for the data source.
func (d *workspaceGroupsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			config.TestIDAttribute: schema.StringAttribute{
				Computed: true,
			},
			dataSourceName: schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Name of the workspace group",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "State of the workspace group",
						},
						"workspace_group_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "ID of the workspace group",
						},
						"firewall_ranges": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "The list of allowed inbound IP addresses",
						},
						"allow_all_traffic": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether or not all traffic is allowed to the workspace group",
						},
						"created_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The timestamp of when the workspace was created",
						},
						"expires_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The timestamp of when the workspace group will expire. At expiration, the workspace group is terminated and all the data is lost.",
						},
						"terminated_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "(If included in the output) The timestamp of when the workspace group was terminated",
						},
						"region_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "ID of the region",
						},
						"update_window": schema.SingleNestedAttribute{
							Computed:            true,
							MarkdownDescription: "Represents information related to an update window",
							Attributes: map[string]schema.Attribute{
								"hour": schema.Int64Attribute{
									Computed:            true,
									MarkdownDescription: "Hour of day - 0 to 23 (UTC)",
								},
								"day": schema.Int64Attribute{
									Computed:            true,
									MarkdownDescription: "Day of week (0-6), starting on Sunday",
								},
							},
						},
					},
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *workspaceGroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	workspaceGroups, err := d.GetV1WorkspaceGroupsWithResponse(ctx, &management.GetV1WorkspaceGroupsParams{})
	if err != nil {
		resp.Diagnostics.AddError(
			"SingleStore API client failed to list workspace groups",
			"An unexpected error occurred when calling SingleStore API workspace groups. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"SingleStore client error: "+err.Error(),
		)

		return
	}

	code := workspaceGroups.StatusCode()
	if code != http.StatusOK {
		resp.Diagnostics.AddError(
			fmt.Sprintf("SingleStore API client returned status code %s while listing workspace groups", http.StatusText(code)),
			"An unsuccessful status code occurred when calling SingleStore API workspace groups. "+
				fmt.Sprintf("Make sure to set the %s value in the configuration or use the %s environment variable. ", config.APIKeyAttribute, config.EnvAPIKey)+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"SingleStore client response body: "+string(workspaceGroups.Body),
		)

		return
	}

	result := workspaceGroupsDataSourceModel{
		ID: types.StringValue(config.TestIDValue),
	}

	for _, workspaceGroup := range util.Deref(workspaceGroups.JSON200) {
		result.WorkspaceGroups = append(result.WorkspaceGroups, workspaceGroupDataSourceModel{
			Name:             types.StringValue(workspaceGroup.Name),
			State:            util.WorkspaceGroupStateStringValue(workspaceGroup.State),
			WorkspaceGroupID: util.UUIDStringValue(workspaceGroup.WorkspaceGroupID),
			FirewallRanges:   util.FirewallRanges(workspaceGroup.FirewallRanges),
			AllowAllTraffic:  util.MaybeBoolValue(workspaceGroup.AllowAllTraffic),
			CreatedAt:        types.StringValue(workspaceGroup.CreatedAt),
			ExpiresAt:        util.MaybeStringValue(workspaceGroup.ExpiresAt),
			TerminatedAt:     util.MaybeStringValue(workspaceGroup.TerminatedAt),
			RegionID:         util.UUIDStringValue(workspaceGroup.RegionID),
			UpdateWindow:     toUpdateWindowDataSourceModel(workspaceGroup.UpdateWindow),
		})
	}

	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *workspaceGroupsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
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
