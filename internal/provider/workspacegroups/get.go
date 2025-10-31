package workspacegroups

import (
	"context"
	"fmt"
	"strings"

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
	ID                       types.String                 `tfsdk:"id"`
	Name                     types.String                 `tfsdk:"name"`
	State                    types.String                 `tfsdk:"state"`
	FirewallRanges           []types.String               `tfsdk:"firewall_ranges"`
	AllowAllTraffic          types.Bool                   `tfsdk:"allow_all_traffic"`
	CreatedAt                types.String                 `tfsdk:"created_at"`
	ExpiresAt                types.String                 `tfsdk:"expires_at"`
	RegionID                 types.String                 `tfsdk:"region_id"`
	CloudProvider            types.String                 `tfsdk:"cloud_provider"`
	RegionName               types.String                 `tfsdk:"region_name"`
	UpdateWindow             *updateWindowDataSourceModel `tfsdk:"update_window"`
	DeploymentType           types.String                 `tfsdk:"deployment_type"`
	OptInPreviewFeature      types.Bool                   `tfsdk:"opt_in_preview_feature"`
	HighAvailabilityTwoZones types.Bool                   `tfsdk:"high_availability_two_zones"`
	OutboundAllowList        types.String                 `tfsdk:"outbound_allow_list"`
}

type workspaceGroupDataSourceSchemaConfig struct {
	computeWorkspaceGroupID    bool
	optionalWorkspaceGroupID   bool
	computeName                bool
	optionalName               bool
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
		MarkdownDescription: "Retrieve a specific workspace group using its name or ID with this data source.",
		Attributes: newWorkspaceGroupDataSourceSchemaAttributes(workspaceGroupDataSourceSchemaConfig{
			optionalWorkspaceGroupID:   true,
			optionalName:               true,
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

	// Validate that exactly one of id or name is provided
	idProvided := !data.ID.IsNull() && !data.ID.IsUnknown()
	nameProvided := !data.Name.IsNull() && !data.Name.IsUnknown()

	if !idProvided && !nameProvided {
		resp.Diagnostics.AddError(
			"Missing identifier",
			"Either 'id' or 'name' must be specified.",
		)

		return
	}

	if idProvided && nameProvided {
		resp.Diagnostics.AddError(
			"Conflicting identifiers",
			"Only one of 'id' or 'name' can be specified, not both.",
		)

		return
	}

	if idProvided {
		readByID(data, ctx, d, resp)

		return
	}

	readByName(data, ctx, d, resp)
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
			Optional:            conf.optionalWorkspaceGroupID,
			MarkdownDescription: "The unique identifier of the workspace group.",
			Validators:          conf.workspaceGroupIDValidators,
		},
		"name": schema.StringAttribute{
			Computed:            conf.computeName,
			Optional:            conf.optionalName,
			MarkdownDescription: "The name of the workspace group.",
		},
		"state": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The state of the workspace group.",
		},
		"firewall_ranges": schema.ListAttribute{
			Computed:            true,
			ElementType:         types.StringType,
			MarkdownDescription: "A list of the allowed inbound IP address ranges.",
		},
		"allow_all_traffic": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Indicates whether all traffic is allowed to reach the workspace group.",
		},
		"created_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The timestamp when the workspace group was created.",
		},
		"expires_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The timestamp when the workspace group will expire. Upon expiration, the workspace group is terminated and all its data is lost.",
		},
		"region_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The unique identifier of the region where the workspace group is located.",
		},
		"cloud_provider": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The name of the cloud provider used to resolve region. Possible values are 'AWS', 'GCP', and 'AZURE'.",
		},
		"region_name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The region code name used to resolve region.",
		},
		"update_window": schema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Details of the scheduled update window for the workspace group. This is the time period during which any updates to the workspace group will occur.",
			Attributes: map[string]schema.Attribute{
				"hour": schema.Int64Attribute{
					Computed:            true,
					MarkdownDescription: "The hour of the day, in 24-hour UTC format (0-23), when the update window starts.",
				},
				"day": schema.Int64Attribute{
					Computed:            true,
					MarkdownDescription: "The day of the week (0-6), where 0 is Sunday and 6 is Saturday, when the update window is scheduled.",
				},
			},
		},
		"deployment_type": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Deployment type of the workspace group.",
		},
		"opt_in_preview_feature": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether 'Opt-in to Preview Features & Updates' is enabled.",
		},
		"high_availability_two_zones": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether deployment across two Availability Zones is enabled.",
		},
		"outbound_allow_list": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The account ID which must be allowed for outbound connections. This is only applicable to AWS provider.",
		},
	}
}

func readByID(data workspaceGroupDataSourceModel, ctx context.Context, d *workspaceGroupsDataSourceGet, resp *datasource.ReadResponse) {
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

	if workspaceGroup.JSON200.State == management.WorkspaceGroupStateFAILED {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Workspace group with the specified ID exists, but is at the %s state", workspaceGroup.JSON200.State),
			config.ContactSupportErrorDetail,
		)

		return
	}

	result := toWorkspaceGroupDataSourceModel(*workspaceGroup.JSON200)

	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func readByName(data workspaceGroupDataSourceModel, ctx context.Context, d *workspaceGroupsDataSourceGet, resp *datasource.ReadResponse) {
	workspaceGroups, err := d.GetV1WorkspaceGroupsWithResponse(ctx, &management.GetV1WorkspaceGroupsParams{})
	if serr := util.StatusOK(workspaceGroups, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	result := util.Filter(util.Deref(workspaceGroups.JSON200), func(wg management.WorkspaceGroup) bool {
		return strings.EqualFold(strings.TrimSpace(wg.Name), strings.TrimSpace(data.Name.ValueString()))
	})

	if len(result) == 0 {
		resp.Diagnostics.AddError(
			"Workspace group not found",
			fmt.Sprintf("No workspace group with the name '%s' was found. Please verify that the name is correct and that the workspace group exists.", data.Name.ValueString()),
		)

		return
	}

	if len(result) > 1 {
		resp.Diagnostics.AddError(
			"Multiple workspace groups found",
			fmt.Sprintf("Multiple workspace groups with the name '%s' were found. Please specify the workspace group ID to uniquely identify the workspace group.", data.Name.ValueString()),
		)

		return
	}

	diags := resp.State.Set(ctx, util.Ptr(toWorkspaceGroupDataSourceModel(result[0])))
	resp.Diagnostics.Append(diags...)
}
