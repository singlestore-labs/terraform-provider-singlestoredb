package workspaces

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
	DataSourceGetName = "workspace"
)

// workspacesDataSourceGet is the data source implementation.
type workspacesDataSourceGet struct {
	management.ClientWithResponsesInterface
}

// workspaceDataSourceModel maps workspace schema data.
type workspaceDataSourceModel struct {
	ID               types.String                       `tfsdk:"id"`
	WorkspaceGroupID types.String                       `tfsdk:"workspace_group_id"`
	Name             types.String                       `tfsdk:"name"`
	State            types.String                       `tfsdk:"state"`
	Size             types.String                       `tfsdk:"size"`
	Suspended        types.Bool                         `tfsdk:"suspended"`
	CreatedAt        types.String                       `tfsdk:"created_at"`
	Endpoint         types.String                       `tfsdk:"endpoint"`
	LastResumedAt    types.String                       `tfsdk:"last_resumed_at"`
	KaiEnabled       types.Bool                         `tfsdk:"kai_enabled"`
	CacheConfig      types.Float32                      `tfsdk:"cache_config"`
	ScaleFactor      types.Float32                      `tfsdk:"scale_factor"`
	AutoScale        *autoScaleResourceModel            `tfsdk:"auto_scale"`
	DeploymentType   types.String                       `tfsdk:"deployment_type"`
	AutoSuspend      *workspaceAutoSuspendResourceModel `tfsdk:"auto_suspend"`
}

type workspaceDataSourceSchemaConfig struct {
	computeWorkspaceID    bool
	optionalWorkspaceID   bool
	computedName          bool
	optionalName          bool
	workspaceIDValidators []validator.String
}

var _ datasource.DataSourceWithConfigure = &workspacesDataSourceGet{}

// NewDataSourceGet is a helper function to simplify the provider implementation.
func NewDataSourceGet() datasource.DataSource {
	return &workspacesDataSourceGet{}
}

// Metadata returns the data source type name.
func (d *workspacesDataSourceGet) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceGetName)
}

// Schema defines the schema for the data source.
func (d *workspacesDataSourceGet) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieve a specific workspace using its ID or name with this data source.",
		Attributes: newWorkspaceDataSourceSchemaAttributes(workspaceDataSourceSchemaConfig{
			optionalWorkspaceID:   true,
			optionalName:          true,
			workspaceIDValidators: []validator.String{util.NewUUIDValidator()},
		}),
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *workspacesDataSourceGet) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data workspaceDataSourceModel
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
func (d *workspacesDataSourceGet) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func newWorkspaceDataSourceSchemaAttributes(conf workspaceDataSourceSchemaConfig) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		config.IDAttribute: schema.StringAttribute{
			Computed:            conf.computeWorkspaceID,
			Optional:            conf.optionalWorkspaceID,
			MarkdownDescription: "The unique identifier of the workspace. Either `id` or `name` must be specified.",
			Validators:          conf.workspaceIDValidators,
		},
		"workspace_group_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The unique identifier of the workspace group that the workspace belongs to. This relationship is established when the workspace is created.",
		},
		"name": schema.StringAttribute{
			Computed:            conf.computedName,
			Optional:            conf.optionalName,
			MarkdownDescription: "The name of the workspace. Either `id` or `name` must be specified.",
		},
		"state": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The current state of the workspace.",
		},
		"created_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The timestamp indicating when the workspace was initially created.",
		},
		"size": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The size of the workspace, represented in workspace size notation, such as 'S-00' or 'S-1'.",
		},
		"suspended": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "A boolean value indicating whether the workspace is currently suspended. If true, the workspace is suspended; if false, the workspace is active.",
		},
		"endpoint": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The endpoint to connect to the workspace.",
		},
		"last_resumed_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The timestamp indicating the most recent time that the workspace was resumed from suspension. If the workspace has never been suspended, this attribute will not be included in the output.",
		},
		"kai_enabled": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether the Kai API is enabled for the workspace.",
		},
		"cache_config": schema.Float32Attribute{
			Computed:            true,
			MarkdownDescription: "Specifies the multiplier for the persistent cache associated with the workspace. It can have one of the following values: 1, 2, or 4.",
		},
		"scale_factor": schema.Float32Attribute{
			Computed:            true,
			MarkdownDescription: "The scale factor specified for the workspace. The scale factor can be 1, 2 or 4.",
		},
		"auto_scale": schema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Specifies the autoscale setting (scale factor) for the workspace.",
			Attributes: map[string]schema.Attribute{
				"max_scale_factor": schema.Float32Attribute{
					Computed:            true,
					MarkdownDescription: "The maximum scale factor allowed for the workspace. It can have the following values: 1, 2, or 4.",
				},
				"sensitivity": schema.StringAttribute{
					Computed:            true,
					MarkdownDescription: "Specifies the sensitivity of the autoscale operation to changes in the workload. It can have the following values: `LOW`, `NORMAL`, or `HIGH`.",
				},
			},
		},
		"deployment_type": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Deployment type of the workspace.",
		},
		"auto_suspend": schema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Represents the current auto suspend settings enabled for this workspace.",
			Attributes: map[string]schema.Attribute{
				"suspend_after_seconds": schema.Float32Attribute{
					Computed:            true,
					MarkdownDescription: "The duration (in seconds) after which the workspace will be suspended if the suspend type is SCHEDULED, or the period of inactivity before automatic suspension if the suspend type is IDLE.",
				},
				"suspend_type": schema.StringAttribute{
					Computed:            true,
					MarkdownDescription: "The type of auto suspend currently enabled.",
				},
			},
		},
	}
}

func toWorkspaceDataSourceModel(workspace management.Workspace) (workspaceDataSourceModel, *util.SummaryWithDetailError) {
	model := workspaceDataSourceModel{
		ID:               util.UUIDStringValue(workspace.WorkspaceID),
		WorkspaceGroupID: util.UUIDStringValue(workspace.WorkspaceGroupID),
		Name:             types.StringValue(workspace.Name),
		State:            util.WorkspaceStateStringValue(workspace.State),
		Size:             types.StringValue(workspace.Size),
		Suspended:        types.BoolValue(workspace.State == management.WorkspaceStateSUSPENDED),
		CreatedAt:        types.StringValue(workspace.CreatedAt),
		Endpoint:         util.MaybeStringValue(workspace.Endpoint),
		LastResumedAt:    util.MaybeStringValue(workspace.LastResumedAt),
		KaiEnabled:       util.MaybeBoolValue(workspace.KaiEnabled),
		CacheConfig:      types.Float32PointerValue(workspace.CacheConfig),
		ScaleFactor:      types.Float32PointerValue(workspace.ScaleFactor),
		AutoScale:        toAutoScaleResourceModel(workspace),
		DeploymentType:   util.StringValueOrNull(workspace.DeploymentType),
		AutoSuspend:      toAutoSuspendResourceModel(workspace),
	}
	if model.CacheConfig.IsNull() || model.CacheConfig.IsUnknown() {
		model.CacheConfig = types.Float32Value(1)
	}

	return model, nil
}

func readByID(data workspaceDataSourceModel, ctx context.Context, d *workspacesDataSourceGet, resp *datasource.ReadResponse) {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.IDAttribute),
			"Invalid workspace ID",
			"The workspace ID should be a valid UUID",
		)

		return
	}

	workspace, err := d.GetV1WorkspacesWorkspaceIDWithResponse(ctx, id, &management.GetV1WorkspacesWorkspaceIDParams{})
	if serr := util.StatusOK(workspace, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	if workspace.JSON200.TerminatedAt != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.IDAttribute),
			fmt.Sprintf("Workspace with the specified ID existed, but got terminated at %s", *workspace.JSON200.TerminatedAt),
			"Make sure to set the workspace ID of the workspace that exists.",
		)

		return
	}

	if workspace.JSON200.State == management.WorkspaceStateFAILED {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Workspace with the specified ID exists, but is at the %s state", workspace.JSON200.State),
			config.ContactSupportErrorDetail,
		)

		return
	}

	result, terr := toWorkspaceDataSourceModel(*workspace.JSON200)
	if terr != nil {
		resp.Diagnostics.AddError(terr.Summary, terr.Detail)

		return
	}

	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func readByName(data workspaceDataSourceModel, ctx context.Context, d *workspacesDataSourceGet, resp *datasource.ReadResponse) {
	// First, get all workspace groups
	workspaceGroups, err := d.GetV1WorkspaceGroupsWithResponse(ctx, &management.GetV1WorkspaceGroupsParams{})
	if serr := util.StatusOK(workspaceGroups, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	var foundWorkspaces []management.Workspace
	targetName := strings.TrimSpace(data.Name.ValueString())

	// For each workspace group, get all workspaces and search for matches
	for _, workspaceGroup := range util.Deref(workspaceGroups.JSON200) {
		workspaces, err := d.GetV1WorkspacesWithResponse(ctx, &management.GetV1WorkspacesParams{
			WorkspaceGroupID: workspaceGroup.WorkspaceGroupID,
		})
		if serr := util.StatusOK(workspaces, err); serr != nil {
			resp.Diagnostics.AddError(
				serr.Summary,
				serr.Detail,
			)

			return
		}

		// Filter workspaces by name (case-insensitive)
		for _, workspace := range util.Deref(workspaces.JSON200) {
			if strings.EqualFold(strings.TrimSpace(workspace.Name), targetName) {
				foundWorkspaces = append(foundWorkspaces, workspace)
			}
		}
	}

	if len(foundWorkspaces) == 0 {
		resp.Diagnostics.AddError(
			"Workspace not found",
			fmt.Sprintf("No workspace with the name '%s' was found in any workspace group. Please verify that the name is correct and that the workspace exists.", data.Name.ValueString()),
		)

		return
	}

	if len(foundWorkspaces) > 1 {
		resp.Diagnostics.AddError(
			"Multiple workspaces found",
			fmt.Sprintf("Multiple workspaces with the name '%s' were found across different workspace groups. Please specify the workspace ID to uniquely identify the workspace.", data.Name.ValueString()),
		)

		return
	}

	result, terr := toWorkspaceDataSourceModel(foundWorkspaces[0])
	if terr != nil {
		resp.Diagnostics.AddError(terr.Summary, terr.Detail)

		return
	}

	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}
