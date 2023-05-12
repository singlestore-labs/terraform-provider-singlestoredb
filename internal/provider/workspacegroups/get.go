package workspacegroups

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	DataSourceGetName = "workspace_group"
)

// workspaceGroupsDataSourceGet is the data source implementation.
type workspaceGroupsDataSourceGet struct {
	management.ClientWithResponsesInterface
}

// workspaceGroupDataSourceModel maps workspace groups schema data.
type workspaceGroupDataSourceModel struct {
	ID              types.String                 `tfsdk:"id"`
	Name            types.String                 `tfsdk:"name"`
	State           types.String                 `tfsdk:"state"`
	FirewallRanges  []types.String               `tfsdk:"firewall_ranges"`
	AllowAllTraffic types.Bool                   `tfsdk:"allow_all_traffic"`
	CreatedAt       types.String                 `tfsdk:"created_at"`
	ExpiresAt       types.String                 `tfsdk:"expires_at"`
	RegionID        types.String                 `tfsdk:"region_id"`
	UpdateWindow    *updateWindowDataSourceModel `tfsdk:"update_window"`
}

type workspaceGroupDataSourceSchemaConfig struct {
	computeWorkspaceGroupID    bool
	requireWorkspaceGroupID    bool
	workspaceGroupIDValidators []validator.String
}

var _ datasource.DataSourceWithConfigure = &workspaceGroupsDataSourceGet{}

// NewDataSourceGet is a helper function to simplify the provider implementation.
func NewDataSourceGet() datasource.DataSource {
	return &workspaceGroupsDataSourceGet{}
}

// Metadata returns the data source type name.
func (d *workspaceGroupsDataSourceGet) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceGetName)
}

// Schema defines the schema for the data source.
func (d *workspaceGroupsDataSourceGet) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: newWorkspaceGroupDataSourceSchemaAttributes(workspaceGroupDataSourceSchemaConfig{
			requireWorkspaceGroupID:    true,
			workspaceGroupIDValidators: []validator.String{util.NewUUIDValidator()},
		}),
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *workspaceGroupsDataSourceGet) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data workspaceGroupDataSourceModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.IDAttribute),
			"Invalid workspace group ID",
			"The workspace group ID should be a valid UUID",
		)

		return
	}

	workspaceGroup, err := d.GetV1WorkspaceGroupsWorkspaceGroupIDWithResponse(ctx, id, &management.GetV1WorkspaceGroupsWorkspaceGroupIDParams{})
	if serr := util.StatusOK(workspaceGroup, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	if workspaceGroup.JSON200.TerminatedAt != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.IDAttribute),
			fmt.Sprintf("Workspace group with the specified ID existed, but got terminated at %s", *workspaceGroup.JSON200.TerminatedAt),
			"Make sure to set the workspace group ID of the workspace group that exists.",
		)

		return
	}

	if workspaceGroup.JSON200.State == management.FAILED {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Workspace group with the specified ID exists, but is at the %s state", workspaceGroup.JSON200.State),
			config.ContactSupportErrorDetail,
		)

		return
	}

	result := toWorkspaceGroupDataSourceModel(*workspaceGroup.JSON200)

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *workspaceGroupsDataSourceGet) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func newWorkspaceGroupDataSourceSchemaAttributes(conf workspaceGroupDataSourceSchemaConfig) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		config.IDAttribute: schema.StringAttribute{
			Computed:            conf.computeWorkspaceGroupID,
			Required:            conf.requireWorkspaceGroupID,
			MarkdownDescription: "ID of the workspace group",
			Validators:          conf.workspaceGroupIDValidators,
		},
		"name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Name of the workspace group",
		},
		"state": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "State of the workspace group",
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
	}
}
