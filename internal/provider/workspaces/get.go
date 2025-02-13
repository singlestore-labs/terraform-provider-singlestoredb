package workspaces

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
	DataSourceGetName = "workspace"
)

// workspacesDataSourceGet is the data source implementation.
type workspacesDataSourceGet struct {
	management.ClientWithResponsesInterface
}

// workspaceDataSourceModel maps workspace schema data.
type workspaceDataSourceModel struct {
	ID               types.String            `tfsdk:"id"`
	WorkspaceGroupID types.String            `tfsdk:"workspace_group_id"`
	Name             types.String            `tfsdk:"name"`
	State            types.String            `tfsdk:"state"`
	Size             types.String            `tfsdk:"size"`
	Suspended        types.Bool              `tfsdk:"suspended"`
	CreatedAt        types.String            `tfsdk:"created_at"`
	Endpoint         types.String            `tfsdk:"endpoint"`
	LastResumedAt    types.String            `tfsdk:"last_resumed_at"`
	KaiEnabled       types.Bool              `tfsdk:"kai_enabled"`
	CacheConfig      types.Float32           `tfsdk:"cache_config"`
	ScaleFactor      types.Float32           `tfsdk:"scale_factor"`
	AutoScale        *autoScaleResourceModel `tfsdk:"auto_scale"`
}

type workspaceDataSourceSchemaConfig struct {
	computeWorkspaceID    bool
	requireWorkspaceID    bool
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
		MarkdownDescription: "Retrieve a specific workspace using its ID with this data source.",
		Attributes: newWorkspaceDataSourceSchemaAttributes(workspaceDataSourceSchemaConfig{
			requireWorkspaceID:    true,
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

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
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
			Required:            conf.requireWorkspaceID,
			MarkdownDescription: "The unique identifier of the workspace.",
			Validators:          conf.workspaceIDValidators,
		},
		"workspace_group_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The unique identifier of the workspace group that the workspace belongs to. This relationship is established when the workspace is created.",
		},
		"name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The name of the workspace.",
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
	}
	if model.CacheConfig.IsNull() || model.CacheConfig.IsUnknown() {
		model.CacheConfig = types.Float32Value(1)
	}

	return model, nil
}
