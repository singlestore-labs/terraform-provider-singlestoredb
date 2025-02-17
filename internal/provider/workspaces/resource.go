package workspaces

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/float32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	ResourceName = "workspace"
)

var (
	_ resource.ResourceWithConfigure   = &workspaceResource{}
	_ resource.ResourceWithModifyPlan  = &workspaceResource{}
	_ resource.ResourceWithImportState = &workspaceResource{}
)

// workspaceResource is the resource implementation.
type workspaceResource struct {
	management.ClientWithResponsesInterface
}

// workspaceResourceModel maps the resource schema data.
type workspaceResourceModel struct {
	ID               types.String                       `tfsdk:"id"`
	WorkspaceGroupID types.String                       `tfsdk:"workspace_group_id"`
	Name             types.String                       `tfsdk:"name"`
	Size             types.String                       `tfsdk:"size"`
	Suspended        types.Bool                         `tfsdk:"suspended"`
	CreatedAt        types.String                       `tfsdk:"created_at"`
	Endpoint         types.String                       `tfsdk:"endpoint"`
	KaiEnabled       types.Bool                         `tfsdk:"kai_enabled"`
	CacheConfig      types.Float32                      `tfsdk:"cache_config"`
	ScaleFactor      types.Float32                      `tfsdk:"scale_factor"`
	AutoScale        *autoScaleResourceModel            `tfsdk:"auto_scale"`
	AutoSuspend      *workspaceAutoSuspendResourceModel `tfsdk:"auto_suspend"`
}

type autoScaleResourceModel struct {
	MaxScaleFactor types.Float32 `tfsdk:"max_scale_factor"`
	Sensitivity    types.String  `tfsdk:"sensitivity"`
}

type workspaceAutoSuspendResourceModel struct {
	SuspendAfterSeconds types.Float32 `tfsdk:"suspend_after_seconds"`
	SuspendType         types.String  `tfsdk:"suspend_type"`
}

// NewResource is a helper function to simplify the provider implementation.
func NewResource() resource.Resource {
	return &workspaceResource{}
}

// Metadata returns the resource type name.
func (r *workspaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, ResourceName)
}

const (
	cacheMultiplierX1 = 1
	cacheMultiplierX2 = 2
	cacheMultiplierX4 = 4
	scaleX1           = 1
	scaleX2           = 2
	scaleX4           = 4
)

// Schema defines the schema for the resource.
func (r *workspaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	autoScaleDefaultValue, _ := basetypes.NewObjectValue(
		map[string]attr.Type{
			"max_scale_factor": basetypes.Float32Type{},
			"sensitivity":      basetypes.StringType{},
		},
		map[string]attr.Value{
			"max_scale_factor": basetypes.NewFloat32Value(scaleX1),
			"sensitivity":      basetypes.NewStringValue(string(management.NORMAL)),
		},
	)
	autoSuspendDefaultValue, _ := basetypes.NewObjectValue(
		map[string]attr.Type{
			"suspend_after_seconds": basetypes.Float32Type{},
			"suspend_type":          basetypes.StringType{},
		},
		map[string]attr.Value{
			"suspend_after_seconds": basetypes.NewFloat32Null(),
			"suspend_type":          basetypes.NewStringValue(string(management.WorkspaceCreateAutoSuspendSuspendTypeDISABLED)),
		},
	)
	resp.Schema = schema.Schema{
		MarkdownDescription: "This resource enables the management of SingleStoreDB workspaces.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the workspace.",
			},
			"workspace_group_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique identifier of the workspace group that the workspace belongs to.",
				Validators:          []validator.String{util.NewUUIDValidator()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name assigned to the workspace.",
			},
			"size": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The size of the workspace, specified in workspace size notation (S-00, S-0, S-1, S-2).",
				Validators:          []validator.String{NewSizeValidator()},
			},
			"suspended": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The status of the workspace. If true, the workspace is suspended.",
				Default:             booldefault.StaticBool(false),
			},
			"created_at": schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The timestamp when the workspace was created.",
			},
			"endpoint": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The endpoint used to connect to the workspace.",
			},
			"kai_enabled": schema.BoolAttribute{
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the Kai API is enabled for the workspace.",
				Default:             booldefault.StaticBool(false),
			},
			"cache_config": schema.Float32Attribute{
				Computed:            true,
				Optional:            true,
				Default:             float32default.StaticFloat32(cacheMultiplierX1),
				Validators:          []validator.Float32{float32validator.OneOf(cacheMultiplierX1, cacheMultiplierX2, cacheMultiplierX4)},
				MarkdownDescription: "Specifies the multiplier for the persistent cache associated with the workspace. It can have one of the following values: 1, 2, or 4. Default is 1.",
			},
			"scale_factor": schema.Float32Attribute{
				Computed:            true,
				Optional:            true,
				Default:             float32default.StaticFloat32(scaleX1),
				Validators:          []validator.Float32{float32validator.OneOf(scaleX1, scaleX2, scaleX4)},
				MarkdownDescription: "Specifies the scale factor for the workspace. The scale factor can be 1, 2 or 4. Default is 1.",
			},
			"auto_scale": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				Default:             objectdefault.StaticValue(autoScaleDefaultValue),
				MarkdownDescription: "Specifies the autoscale setting (scale factor) for the workspace.",
				Attributes: map[string]schema.Attribute{
					"max_scale_factor": schema.Float32Attribute{
						Optional:            true,
						Computed:            true,
						Default:             float32default.StaticFloat32(scaleX1),
						Validators:          []validator.Float32{float32validator.OneOf(scaleX1, scaleX2, scaleX4)},
						MarkdownDescription: "The maximum scale factor allowed for the workspace. It can have the following values: 1, 2, or 4. To disable autoscaling, set to 1. Default is 1.",
					},
					"sensitivity": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Specifies the sensitivity of the autoscale operation to changes in the workload. It can have the following values: `LOW`, `NORMAL`, or `HIGH`. Default is `NORMAL`.",
						Default:             stringdefault.StaticString(string(management.NORMAL)),
						Validators: []validator.String{
							stringvalidator.OneOf(string(management.LOW), string(management.NORMAL), string(management.HIGH)),
						},
					},
				},
			},
			"auto_suspend": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Auto suspend settings for the workspace.",
				Default:             objectdefault.StaticValue(autoSuspendDefaultValue),
				Attributes: map[string]schema.Attribute{
					"suspend_after_seconds": schema.Float32Attribute{
						Optional:            true,
						MarkdownDescription: "When to suspend the workspace, according to the suspend type chosen.",
					},
					"suspend_type": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "The auto suspend mode for the workspace can have the values `IDLE`, `SCHEDULED`, or `DISABLED` (to create the workspace with no auto suspend settings). Default is `DISABLED`.",
						Default:             stringdefault.StaticString(string(management.WorkspaceCreateAutoSuspendSuspendTypeDISABLED)),
						Validators: []validator.String{
							stringvalidator.OneOf(string(management.WorkspaceCreateAutoSuspendSuspendTypeDISABLED), string(management.WorkspaceCreateAutoSuspendSuspendTypeIDLE), string(management.WorkspaceCreateAutoSuspendSuspendTypeSCHEDULED)),
						},
					},
				},
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *workspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workspaceResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Suspended.ValueBool() {
		resp.Diagnostics.AddAttributeError(
			path.Root("suspended"),
			"Cannot suspend a workspace during creation",
			"Either set value to false or omit the field.",
		)

		return
	}

	if err := validateAutoSuspendConfig(&plan); err != nil {
		resp.Diagnostics.AddError(
			err.Summary,
			err.Detail,
		)

		return
	}

	if err := validateAutoScaleConfig(&plan); err != nil {
		resp.Diagnostics.AddError(
			err.Summary,
			err.Detail,
		)

		return
	}

	workspaceCreateResponse, err := r.PostV1WorkspacesWithResponse(ctx, management.PostV1WorkspacesJSONRequestBody{
		Name:             plan.Name.ValueString(),
		Size:             util.MaybeString(plan.Size),
		WorkspaceGroupID: uuid.MustParse(plan.WorkspaceGroupID.String()),
		EnableKai:        util.MaybeBool(plan.KaiEnabled),
		CacheConfig:      util.MaybeFloat32(plan.CacheConfig),
		ScaleFactor:      util.MaybeFloat32(plan.ScaleFactor),
		AutoSuspend:      toCreateAutoSuspend(plan),
	})
	if serr := util.StatusOK(workspaceCreateResponse, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	// Execute PATCH call to proceed autoScale
	if !plan.AutoScale.MaxScaleFactor.Equal(types.Float32Value(scaleX1)) {
		workspace, err := r.PatchV1WorkspacesWorkspaceIDWithResponse(ctx, workspaceCreateResponse.JSON200.WorkspaceID,
			management.PatchV1WorkspacesWorkspaceIDJSONRequestBody{
				AutoScale: toAutoScale(plan),
			},
		)
		if serr := util.StatusOK(workspace, err); serr != nil {
			resp.Diagnostics.AddError(
				serr.Summary,
				serr.Detail,
			)

			return
		}
	}

	w, werr := wait(ctx, r.ClientWithResponsesInterface, workspaceCreateResponse.JSON200.WorkspaceID, config.WorkspaceCreationTimeout,
		waitConditionState(management.WorkspaceStateACTIVE),
	)
	if werr != nil {
		resp.Diagnostics.AddError(
			werr.Summary,
			werr.Detail,
		)

		return
	}

	result := toWorkspaceResourceModel(w)
	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *workspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workspaceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := uuid.MustParse(state.ID.ValueString())

	workspace, err := r.GetV1WorkspacesWorkspaceIDWithResponse(ctx, id,
		&management.GetV1WorkspacesWorkspaceIDParams{},
	)
	if serr := util.StatusOK(workspace, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	if workspace.JSON200.State == management.WorkspaceStateTERMINATED {
		resp.State.RemoveResource(ctx)

		return // The resource got terminated externally, deleting it from the state file to recreate.
	}

	if workspace.JSON200.State != management.WorkspaceStateACTIVE &&
		workspace.JSON200.State != management.WorkspaceStateSUSPENDED {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Workspace %s state is %s while it should be %s or %s", state.ID.ValueString(), workspace.JSON200.State, management.WorkspaceStateACTIVE, management.WorkspaceStateSUSPENDED),
			"An unexpected workspace state.\n\n"+
				config.ContactSupportLaterErrorDetail,
		)

		return
	}

	state = toWorkspaceResourceModel(*workspace.JSON200)
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *workspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state workspaceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan workspaceResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var uerr *util.SummaryWithDetailError
	state, uerr = applyWorkspaceConfigOrToggleSuspension(ctx, r.ClientWithResponsesInterface, state, plan)
	if uerr != nil {
		resp.Diagnostics.AddError(
			uerr.Summary,
			uerr.Detail,
		)

		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *workspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workspaceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceDeleteResponse, err := r.DeleteV1WorkspacesWorkspaceIDWithResponse(ctx, uuid.MustParse(state.ID.ValueString()))
	if serr := util.StatusOK(workspaceDeleteResponse, err, util.ReturnNilOnNotFound); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}
}

// Configure adds the provider configured client to the resource.
func (r *workspaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	r.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

// ModifyPlan emits an error if a required yet immutable field changes or if incompatible state is set.
//
// `RequiresReplace` is not used because deleting a workspace result in losing database attachments.
func (r *workspaceResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var state *workspaceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *workspaceResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}

	if !plan.Name.Equal(state.Name) {
		resp.Diagnostics.AddError("Cannot update workspace name",
			"To prevent accidental deletion of the databases that are attached to the workspace, updating the name is not permitted. "+
				"Please explicitly delete the workspace before changing its name.")

		return
	}

	if !plan.KaiEnabled.Equal(state.KaiEnabled) {
		resp.Diagnostics.AddError("Cannot change the kai_enabled configuration for the workspace",
			"Changing the kai_enabled configuration is currently not supported.")

		return
	}

	if !plan.WorkspaceGroupID.Equal(state.WorkspaceGroupID) {
		resp.Diagnostics.AddError("Cannot update workspace group ID",
			"To prevent accidental deletion of the databases that are attached to the workspace, updating the workspace group ID is not permitted. "+
				"Please explicitly delete the workspace before changing its workspace group ID.")

		return
	}

	if err := validateSuspendedAndConfigChanges(state, plan); err != nil {
		resp.Diagnostics.AddError(err.Summary, err.Detail)

		return
	}
}

// ImportState results in Terraform managing the resource that was not previously managed.
func (r *workspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root(config.IDAttribute), req, resp)
}

func toWorkspaceResourceModel(workspace management.Workspace) workspaceResourceModel {
	model := workspaceResourceModel{
		ID:               util.UUIDStringValue(workspace.WorkspaceID),
		WorkspaceGroupID: util.UUIDStringValue(workspace.WorkspaceGroupID),
		Name:             types.StringValue(workspace.Name),
		Size:             types.StringValue(workspace.Size),
		Suspended:        types.BoolValue(workspace.State == management.WorkspaceStateSUSPENDED),
		CreatedAt:        types.StringValue(workspace.CreatedAt),
		Endpoint:         util.MaybeStringValue(workspace.Endpoint),
		KaiEnabled:       types.BoolValue(util.Deref(workspace.KaiEnabled)),
		CacheConfig:      types.Float32PointerValue(workspace.CacheConfig),
		ScaleFactor:      types.Float32PointerValue(workspace.ScaleFactor),
		AutoScale:        toAutoScaleResourceModel(workspace),
		AutoSuspend:      toAutoSuspendResourceModel(workspace),
	}
	if model.CacheConfig.IsNull() || model.CacheConfig.IsUnknown() {
		model.CacheConfig = types.Float32Value(1)
	}

	return model
}

func toCreateAutoSuspend(plan workspaceResourceModel) *struct {
	SuspendAfterSeconds *float32                                          `json:"suspendAfterSeconds,omitempty"`
	SuspendType         *management.WorkspaceCreateAutoSuspendSuspendType `json:"suspendType,omitempty"`
} {
	return &struct {
		SuspendAfterSeconds *float32                                          `json:"suspendAfterSeconds,omitempty"`
		SuspendType         *management.WorkspaceCreateAutoSuspendSuspendType `json:"suspendType,omitempty"`
	}{
		SuspendAfterSeconds: util.MaybeFloat32(plan.AutoSuspend.SuspendAfterSeconds),
		SuspendType:         util.WorkspaceCreateAutoSuspendSuspendTypeString(plan.AutoSuspend.SuspendType),
	}
}

func toAutoSuspendResourceModel(ws management.Workspace) *workspaceAutoSuspendResourceModel {
	if ws.AutoSuspend == nil {
		return &workspaceAutoSuspendResourceModel{
			SuspendType: types.StringValue(string(management.WorkspaceCreateAutoSuspendSuspendTypeDISABLED)),
		}
	}
	var suspendAfterSeconds *float32
	if ws.AutoSuspend.SuspendType == management.WorkspaceAutoSuspendSuspendTypeIDLE {
		suspendAfterSeconds = ws.AutoSuspend.IdleAfterSeconds
	} else if ws.AutoSuspend.SuspendType == management.WorkspaceAutoSuspendSuspendTypeSCHEDULED {
		suspendAfterSeconds = ws.AutoSuspend.ScheduledAfterSeconds
	}

	return &workspaceAutoSuspendResourceModel{
		SuspendAfterSeconds: types.Float32PointerValue(suspendAfterSeconds),
		SuspendType:         util.StringValueOrNull(&ws.AutoSuspend.SuspendType),
	}
}

func toAutoScale(plan workspaceResourceModel) *struct {
	MaxScaleFactor *float32                                        `json:"maxScaleFactor,omitempty"`
	Sensitivity    *management.WorkspaceUpdateAutoScaleSensitivity `json:"sensitivity,omitempty"`
} {
	// If MaxScaleFactor = 1 ignore sensitivity to disable autoscaling
	var sensitivity *management.WorkspaceUpdateAutoScaleSensitivity
	if !plan.AutoScale.MaxScaleFactor.Equal(types.Float32Value(scaleX1)) {
		sensitivity = util.WorkspaceAutoScaleSensitivityString(plan.AutoScale.Sensitivity)
	}

	return &struct {
		MaxScaleFactor *float32                                        `json:"maxScaleFactor,omitempty"`
		Sensitivity    *management.WorkspaceUpdateAutoScaleSensitivity `json:"sensitivity,omitempty"`
	}{
		MaxScaleFactor: util.MaybeFloat32(plan.AutoScale.MaxScaleFactor),
		Sensitivity:    sensitivity,
	}
}

func toAutoScaleResourceModel(ws management.Workspace) *autoScaleResourceModel {
	if ws.AutoScale == nil {
		return &autoScaleResourceModel{
			MaxScaleFactor: types.Float32Value(scaleX1),
			Sensitivity:    types.StringValue(string(management.NORMAL)),
		}
	}

	return &autoScaleResourceModel{
		MaxScaleFactor: types.Float32Value(ws.AutoScale.MaxScaleFactor),
		Sensitivity:    util.StringValueOrNull(ws.AutoScale.Sensitivity),
	}
}

func validateSuspendedAndConfigChanges(state, plan *workspaceResourceModel) *util.SummaryWithDetailError {
	if err := validateAutoScaleConfig(plan); err != nil {
		return err
	}

	if err := validateAutoSuspendConfig(plan); err != nil {
		return err
	}

	suspendedChanged := !plan.Suspended.Equal(state.Suspended)
	isSuspended := plan.Suspended.ValueBool()

	otherConfigChanged := hasGeneralConfigChanged(*state, *plan)

	// Changing both suspended and other configurations is prohibited.
	if otherConfigChanged && suspendedChanged {
		return &util.SummaryWithDetailError{
			Summary: "Cannot update both the suspension state and other configurations (such as size, cache_config, scale_factor, auto_scale or auto_suspend) at the same time",
			Detail:  "To avoid an inconsistent state, either suspend the workspace or update the other configurations (such as size, cache_config, scale_factor, auto_scale or auto_suspend).",
		}
	}

	// If a workspace is suspended, other configurations is prohibited.
	if otherConfigChanged && isSuspended {
		return &util.SummaryWithDetailError{
			Summary: "Cannot update the configuration (such as size, cache_config, scale_factor, auto_scale or auto_suspend) for a suspended workspace.",
			Detail:  "Resume the workspace by setting suspended to false before updating the configuration (such as size, cache_config, scale_factor, auto_scale or auto_suspend).",
		}
	}

	return nil
}

func validateAutoSuspendConfig(plan *workspaceResourceModel) *util.SummaryWithDetailError {
	if plan.AutoSuspend.SuspendType.Equal(types.StringValue(string(management.WorkspaceCreateAutoSuspendSuspendTypeDISABLED))) &&
		!plan.AutoSuspend.SuspendAfterSeconds.IsNull() {
		return &util.SummaryWithDetailError{
			Summary: "Invalid auto_suspend configuration.",
			Detail:  "If suspend_type is set to DISABLED, the suspend_after_seconds parameter is not allowed.",
		}
	}

	return nil
}

func validateAutoScaleConfig(plan *workspaceResourceModel) *util.SummaryWithDetailError {
	// If Sensitivity is not default and MaxScaleFactor is set to 1 throw error
	if !plan.AutoScale.Sensitivity.Equal(types.StringValue(string(management.NORMAL))) &&
		plan.AutoScale.MaxScaleFactor.Equal(types.Float32Value(scaleX1)) {
		return &util.SummaryWithDetailError{
			Summary: "Invalid auto_scale configuration.",
			Detail:  "If max_scale_factor is set to 1, the sensitivity parameter (if not set to its default value) is not allowed.",
		}
	}

	return nil
}
