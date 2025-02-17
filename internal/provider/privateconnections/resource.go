package privateconnections

import (
	"context"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	ResourceName = "private_connection"
)

var (
	_ resource.ResourceWithConfigure   = &privateConnectionResource{}
	_ resource.ResourceWithModifyPlan  = &privateConnectionResource{}
	_ resource.ResourceWithImportState = &privateConnectionResource{}
)

// privateConnectionResource is the resource implementation.
type privateConnectionResource struct {
	management.ClientWithResponsesInterface
}

// privateConnectionModel maps the resource schema data.
type PrivateConnectionModel struct {
	ID                types.String  `tfsdk:"id"`
	ActiveAt          types.String  `tfsdk:"active_at"`
	AllowList         types.String  `tfsdk:"allow_list"`
	KaiEndpointID     types.String  `tfsdk:"kai_endpoint_id"`
	CreatedAt         types.String  `tfsdk:"created_at"`
	DeletedAt         types.String  `tfsdk:"deleted_at"`
	Endpoint          types.String  `tfsdk:"endpoint"`
	OutboundAllowList types.String  `tfsdk:"outbound_allow_list"`
	ServiceName       types.String  `tfsdk:"service_name"`
	Status            types.String  `tfsdk:"status"`
	SQLPort           types.Float32 `tfsdk:"sql_port"`
	Type              types.String  `tfsdk:"type"`
	WebsocketsPort    types.Float32 `tfsdk:"web_socket_port"`
	UpdatedAt         types.String  `tfsdk:"updated_at"`
	WorkspaceGroupID  types.String  `tfsdk:"workspace_group_id"`
	WorkspaceID       types.String  `tfsdk:"workspace_id"`
}

const (
	defaultSQLPort       float32 = 3306
	defaultWebsocketPort float32 = 443
)

// NewResource is a helper function to simplify the provider implementation.
func NewResource() resource.Resource {
	return &privateConnectionResource{}
}

// Metadata returns the resource type name.
func (r *privateConnectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, ResourceName)
}

// Schema defines the schema for the resource.
func (r *privateConnectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage SingleStoreDB workspace private connections with this resource.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the private connection.",
			},
			"active_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The timestamp of when the private connection became active.",
			},
			"allow_list": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The private connection allow list. This is the account ID for AWS,  subscription ID for Azure, and the project name GCP.",
			},
			"kai_endpoint_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "VPC Endpoint ID for AWS.",
			},
			"created_at": schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The timestamp of when the private connection was created.",
			},
			"deleted_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The timestamp of when the private connection was deleted.",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The timestamp of when the private connection was updated.",
			},
			"endpoint": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The service endpoint.",
			},
			"outbound_allow_list": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The account ID which must be allowed for outbound connections.",
			},
			"service_name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The name of the private connection service.",
			},
			"type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The private connection type.",
				Validators: []validator.String{
					stringvalidator.OneOf(string(management.PrivateConnectionCreateTypeINBOUND), string(management.PrivateConnectionCreateTypeOUTBOUND)),
				},
				Default: stringdefault.StaticString(string(management.PrivateConnectionCreateTypeINBOUND)),
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The status of the private connection.",
			},
			"sql_port": schema.Float32Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The SQL port.",
				Default:             float32default.StaticFloat32(defaultSQLPort),
			},
			"web_socket_port": schema.Float32Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The websockets port.",
				Default:             float32default.StaticFloat32(defaultWebsocketPort),
			},
			"workspace_group_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the workspace group containing the private connection.",
			},
			"workspace_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The ID of the workspace to connect with.",
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *privateConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PrivateConnectionModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	verr := ValidatePrivateConnection(plan, false)
	if verr != nil {
		resp.Diagnostics.AddError(
			verr.Summary,
			verr.Detail,
		)

		return
	}

	privateConnectionType, err := util.PrivateConnectionTypeString(plan.Type)
	if err != nil {
		resp.Diagnostics.AddError(
			err.Error(),
			err.Error(),
		)

		return
	}

	var workspaceID *uuid.UUID
	if !plan.WorkspaceID.IsNull() {
		parsedID := uuid.MustParse(plan.WorkspaceID.String())
		workspaceID = &parsedID
	}

	// If custom socket are not enabled, sqlPort and websocketPort must be empty
	var sqlPort *float32
	if plan.SQLPort.ValueFloat32() != defaultSQLPort {
		sqlPort = util.MaybeFloat32(plan.SQLPort)
	}

	var websocketPort *float32
	if plan.WebsocketsPort.ValueFloat32() != defaultWebsocketPort {
		websocketPort = util.MaybeFloat32(plan.WebsocketsPort)
	}

	privateConnectionCreateResponse, err := r.PostV1PrivateConnectionsWithResponse(ctx, management.PostV1PrivateConnectionsJSONRequestBody{
		AllowList:        util.MaybeString(plan.AllowList),
		KaiEndpointID:    util.MaybeString(plan.KaiEndpointID),
		ServiceName:      util.MaybeString(plan.ServiceName),
		SqlPort:          sqlPort,
		Type:             &privateConnectionType,
		WebsocketsPort:   websocketPort,
		WorkspaceGroupID: uuid.MustParse(plan.WorkspaceGroupID.String()),
		WorkspaceID:      workspaceID,
	})

	if serr := util.StatusOK(privateConnectionCreateResponse, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	id := privateConnectionCreateResponse.JSON200.PrivateConnectionID
	con, werr := WaitPrivateConnectionStatus(ctx, r.ClientWithResponsesInterface, id, waitConditionStatus(management.PrivateConnectionStatusACTIVE))
	if werr != nil {
		resp.Diagnostics.AddError(
			werr.Summary,
			werr.Detail,
		)

		return
	}

	result, terr := toPrivateConnectionModel(con)

	if terr != nil {
		resp.Diagnostics.AddError(terr.Summary, terr.Detail)

		return
	}

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *privateConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PrivateConnectionModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	privateConnection, err := r.GetV1PrivateConnectionsConnectionIDWithResponse(ctx,
		uuid.MustParse(state.ID.ValueString()),
		&management.GetV1PrivateConnectionsConnectionIDParams{},
	)
	if serr := util.StatusOK(privateConnection, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	if privateConnection.JSON200.Status != nil && *privateConnection.JSON200.Status == management.PrivateConnectionStatusDELETED {
		resp.State.RemoveResource(ctx)

		return // The resource got deleted externally, deleting it from the state file to recreate.
	}

	state, terr := toPrivateConnectionModel(*privateConnection.JSON200)
	if terr != nil {
		resp.Diagnostics.AddError(terr.Summary, terr.Detail)

		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *privateConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state PrivateConnectionModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan PrivateConnectionModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	verr := ValidatePrivateConnection(plan, true)
	if verr != nil {
		resp.Diagnostics.AddError(
			verr.Summary,
			verr.Detail,
		)

		return
	}

	if plan.AllowList.Equal(state.AllowList) {
		return
	}

	id := uuid.MustParse(plan.ID.ValueString())

	privateConnectionUpdateResponse, err := r.PatchV1PrivateConnectionsConnectionIDWithResponse(ctx, id,
		management.PrivateConnectionUpdate{
			AllowList: util.MaybeString(plan.AllowList),
		},
	)
	if serr := util.StatusOK(privateConnectionUpdateResponse, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	privateConnection, werr := WaitPrivateConnectionStatus(ctx, r.ClientWithResponsesInterface, id, waitConditionAllowList(util.ToString(plan.AllowList)))
	if werr != nil {
		resp.Diagnostics.AddError(
			werr.Summary,
			werr.Detail,
		)

		return
	}

	result, terr := toPrivateConnectionModel(privateConnection)

	if terr != nil {
		resp.Diagnostics.AddError(terr.Summary, terr.Detail)

		return
	}

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *privateConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PrivateConnectionModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	privateConnectionDeleteResponse, err := r.DeleteV1PrivateConnectionsConnectionIDWithResponse(ctx,
		uuid.MustParse(state.ID.ValueString()),
	)
	if serr := util.StatusOK(privateConnectionDeleteResponse, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}
}

// Configure adds the provider configured client to the resource.
func (r *privateConnectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	r.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

// ModifyPlan emits an error if a required yet immutable field changes or if incompatible state is set.
func (r *privateConnectionResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var state *PrivateConnectionModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *PrivateConnectionModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}

	verr := ValidatePrivateConnectionModifyPlan(*plan, *state)
	if verr != nil {
		resp.Diagnostics.AddError(
			verr.Summary,
			verr.Detail,
		)

		return
	}
}

// ImportState results in Terraform managing the resource that was not previously managed.
func (r *privateConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root(config.IDAttribute), req, resp)
}

func toPrivateConnectionModel(privateConnection management.PrivateConnection) (PrivateConnectionModel, *util.SummaryWithDetailError) {
	var kaiEndpointID types.String
	if privateConnection.AllowedPrivateLinkIDs != nil && len(*privateConnection.AllowedPrivateLinkIDs) > 0 {
		kaiEndpointID = types.StringValue((*privateConnection.AllowedPrivateLinkIDs)[0])
	}
	model := PrivateConnectionModel{
		ID:                util.UUIDStringValue(privateConnection.PrivateConnectionID),
		ActiveAt:          util.MaybeStringValue(privateConnection.ActiveAt),
		AllowList:         util.MaybeStringValue(privateConnection.AllowList),
		CreatedAt:         util.MaybeStringValue(privateConnection.CreatedAt),
		DeletedAt:         util.MaybeStringValue(privateConnection.DeletedAt),
		Status:            util.StringValueOrNull(privateConnection.Status),
		OutboundAllowList: util.MaybeStringValue(privateConnection.OutboundAllowList),
		ServiceName:       util.MaybeStringValue(privateConnection.ServiceName),
		Endpoint:          util.MaybeStringValue(privateConnection.Endpoint),
		KaiEndpointID:     kaiEndpointID,
		Type:              util.StringValueOrNull(privateConnection.Type),
		UpdatedAt:         util.MaybeStringValue(privateConnection.UpdatedAt),
		WorkspaceGroupID:  util.UUIDStringValue(privateConnection.WorkspaceGroupID),
		WorkspaceID:       util.MaybeUUIDStringValue(privateConnection.WorkspaceID),
		SQLPort:           types.Float32PointerValue(privateConnection.SqlPort),
		WebsocketsPort:    types.Float32PointerValue(privateConnection.WebsocketsPort),
	}
	if model.SQLPort.IsNull() || model.SQLPort.IsUnknown() {
		model.SQLPort = types.Float32Value(defaultSQLPort)
	}
	if model.WebsocketsPort.IsNull() || model.WebsocketsPort.IsUnknown() {
		model.WebsocketsPort = types.Float32Value(defaultWebsocketPort)
	}

	return model, nil
}
