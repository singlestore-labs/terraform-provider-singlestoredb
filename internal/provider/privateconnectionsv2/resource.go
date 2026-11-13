package privateconnectionsv2

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const ResourceName = "private_connection_v2"

var (
	_ resource.ResourceWithConfigure   = &privateConnectionResource{}
	_ resource.ResourceWithModifyPlan  = &privateConnectionResource{}
	_ resource.ResourceWithImportState = &privateConnectionResource{}
)

type privateConnectionResource struct {
	management.ClientWithResponsesInterface
}

func NewResource() resource.Resource {
	return &privateConnectionResource{}
}

func (r *privateConnectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, ResourceName)
}

func (r *privateConnectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage a private connection for a SingleStoreDB cluster with the v2 Management API. " +
			"Import reads the connection by ID. `cluster_id` comes from that response, so a response that omits it cannot be imported. " +
			"Changing `cluster_id`, `type`, `service_name`, `kai_endpoint_id`, `sql_port`, or `web_socket_port` replaces the connection. " +
			"`allow_list` and `supported_cross_regions` can be updated in place.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the private connection.",
			},
			"cluster_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the cluster to connect with.",
				Validators:          []validator.String{util.NewUUIDValidator()},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The private connection type, `INBOUND` or `OUTBOUND`.",
				Validators: []validator.String{
					stringvalidator.OneOf(string(management.PrivateConnectionCreateV2TypeINBOUND), string(management.PrivateConnectionCreateV2TypeOUTBOUND)),
				},
				Default: stringdefault.StaticString(string(management.PrivateConnectionCreateV2TypeINBOUND)),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"allow_list": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The private connection allow list. This is the account ID for AWS, the subscription ID for Azure, and the project name for GCP. Required for an inbound connection unless `kai_endpoint_id` is set. Not allowed for an outbound connection.",
			},
			"kai_endpoint_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "VPC Endpoint ID for an inbound SingleStore Kai connection. Cannot be combined with `allow_list`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The name of the private connection service. Required for an outbound connection. The API assigns it for an inbound connection.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"sql_port": schema.Float32Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The SQL port. Defaults to 3306. A custom port is sent only when it is not 3306, because the API rejects a port when custom sockets are not enabled.",
				Default:             float32default.StaticFloat32(defaultSQLPort),
				PlanModifiers: []planmodifier.Float32{
					float32planmodifier.RequiresReplace(),
				},
			},
			"web_socket_port": schema.Float32Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The websockets port. Defaults to 443. A custom port is sent only when it is not 443, because the API rejects a port when custom sockets are not enabled.",
				Default:             float32default.StaticFloat32(defaultWebsocketPort),
				PlanModifiers: []planmodifier.Float32{
					float32planmodifier.RequiresReplace(),
				},
			},
			"supported_cross_regions": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Additional regions allowed to reach this private connection. An update replaces the list. Omit the argument, or set it to an empty list, to remove every region.",
			},
			"active_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The timestamp of when the private connection became active.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The timestamp of when the private connection was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The status of the private connection.",
			},
		},
	}
}

func (r *privateConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	plan, ok := createPlan(ctx, req, &resp.Diagnostics)
	if !ok {
		return
	}

	clusterID, perr := parseUUID(plan.ClusterID.ValueString(), "cluster ID")
	if perr != nil {
		resp.Diagnostics.AddAttributeError(path.Root("cluster_id"), perr.Summary, perr.Detail)

		return
	}

	connectionType, terr := connectionTypeValue(plan.Type)
	if terr != nil {
		resp.Diagnostics.AddError(terr.Summary, terr.Detail)

		return
	}

	created, err := r.PostV2PrivateConnectionsWithResponse(ctx, createRequest(ctx, plan, clusterID, connectionType))
	if serr := util.StatusOK(created, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}
	if created.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Private connection create returned an unreadable response",
			fmt.Sprintf("POST /v2/privateConnections returned status code OK without a JSON body (Content-Type %q). A private connection may have been created. Check the SingleStore Portal before applying again.",
				responseContentType(created.HTTPResponse)),
		)

		return
	}

	id := created.JSON200.PrivateConnectionID
	// Record the connection before waiting. Terraform keeps a resource only when
	// Create stores it, so a wait failure can still refresh or destroy this
	// connection instead of creating another one.
	stored := plan
	stored.ID = util.UUIDStringValue(id)
	clearUnknown(&stored)
	resp.Diagnostics.Append(resp.State.Set(ctx, &stored)...)
	if resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError(
			"Private connection created but not stored in state",
			fmt.Sprintf("Private connection %s was created and is not tracked. Import it, or delete it in the SingleStore Portal, before applying again.", id),
		)

		return
	}

	conn, werr := wait(ctx, r.ClientWithResponsesInterface, id, config.PrivateConnectionCreationTimeout, waitUntilActive, createConditions(ctx, plan)...)
	if werr != nil {
		resp.Diagnostics.AddError(werr.Summary, werr.Detail)

		return
	}

	result := toPrivateConnectionModel(ctx, conn)
	fillOmitted(ctx, &result, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &result)...)
}

func (r *privateConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state privateConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, perr := parseUUID(state.ID.ValueString(), "private connection ID")
	if perr != nil {
		resp.Diagnostics.AddAttributeError(path.Root(config.IDAttribute), perr.Summary, perr.Detail)

		return
	}

	conn, notFound, err := readConnection(ctx, r.ClientWithResponsesInterface, id)
	if err != nil {
		resp.Diagnostics.AddError(err.Summary, err.Detail)

		return
	}
	if notFound || util.Deref(conn.Status) == management.ClusterPrivateConnectionStatusDELETED {
		resp.State.RemoveResource(ctx)

		return
	}

	if util.Deref(conn.Status) == management.ClusterPrivateConnectionStatusPENDING {
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("Private connection %s status is %s", id, management.ClusterPrivateConnectionStatusPENDING),
			"The private connection is not active yet. The next apply waits until it is active.",
		)
	}

	result := toPrivateConnectionModel(ctx, *conn)
	fillOmitted(ctx, &result, state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &result)...)
}

func (r *privateConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state privateConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan privateConnectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := configuredServiceNameError(ctx, req.Config, plan, &resp.Diagnostics); err != nil {
		r.keepPriorState(ctx, resp, state, err.Summary, err.Detail)

		return
	}
	if resp.Diagnostics.HasError() {
		r.keepPriorState(ctx, resp, state, "Failed to read private connection configuration", "The private connection configuration could not be read.")

		return
	}
	if err := validateConfiguration(plan, true); err != nil {
		r.keepPriorState(ctx, resp, state, err.Summary, err.Detail)

		return
	}

	id, perr := parseUUID(plan.ID.ValueString(), "private connection ID")
	if perr != nil {
		r.keepPriorState(ctx, resp, state, perr.Summary, perr.Detail)

		return
	}

	body, changed := updateRequest(ctx, state, plan)
	if !changed {
		r.applyUnchanged(ctx, resp, state, id)

		return
	}

	updated, err := r.PatchV2PrivateConnectionsConnectionIDWithResponse(ctx, id, body)
	if serr := util.StatusOK(updated, err); serr != nil {
		// State starts as the plan. Put the previous values back so the next plan retries.
		r.keepPriorState(ctx, resp, state, serr.Summary, serr.Detail)

		return
	}

	conn, werr := wait(ctx, r.ClientWithResponsesInterface, id, config.PrivateConnectionCreationTimeout, waitUntilActive, updateConditions(ctx, state, plan)...)
	if werr != nil {
		r.keepPriorState(ctx, resp, state, werr.Summary, werr.Detail)

		return
	}

	result := toPrivateConnectionModel(ctx, conn)
	fillOmitted(ctx, &result, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &result)...)
}

func (r *privateConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state privateConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, perr := parseUUID(state.ID.ValueString(), "private connection ID")
	if perr != nil {
		resp.Diagnostics.AddAttributeError(path.Root(config.IDAttribute), perr.Summary, perr.Detail)

		return
	}

	deleted, err := r.DeleteV2PrivateConnectionsConnectionIDWithResponse(ctx, id)
	if serr := util.StatusOK(deleted, err, util.ReturnNilOnNotFound); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	if _, werr := wait(ctx, r.ClientWithResponsesInterface, id, config.PrivateConnectionCreationTimeout, waitUntilDeleted); werr != nil {
		resp.Diagnostics.AddError(werr.Summary, werr.Detail)
	}
}

func (r *privateConnectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func (r *privateConnectionResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan privateConnectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config privateConnectionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// service_name is computed, so an omitted value is unknown in the plan.
	// The configuration shows whether the user set it.
	if err := validateServiceNameConfig(config, plan); err != nil {
		resp.Diagnostics.AddError(err.Summary, err.Detail)

		return
	}

	// Known-invalid configuration fails here. Unknown arguments are checked again at apply.
	if err := validateConfiguration(plan, false); err != nil {
		resp.Diagnostics.AddError(err.Summary, err.Detail)

		return
	}

	// Read warns that a PENDING connection is waited for on the next apply.
	// Marking status unknown plans an update, and Update then waits.
	if !req.State.Raw.IsNull() {
		var state privateConnectionModel
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if isPending(state) {
			plan.Status = types.StringUnknown()
		}
	}

	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

func isPending(model privateConnectionModel) bool {
	return model.Status.ValueString() == string(management.ClusterPrivateConnectionStatusPENDING)
}

func (r *privateConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	util.ImportStatePassthroughID(ctx, req, resp)
}

// keepPriorState stores the last saved values after a failed update.
// resp.State starts as the plan and is saved even when Update returns an error,
// so leaving it unchanged would record the failed change as applied.
func (r *privateConnectionResource) keepPriorState(ctx context.Context, resp *resource.UpdateResponse, state privateConnectionModel, summary, detail string) {
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	resp.Diagnostics.AddError(summary, detail)
}

// applyUnchanged handles an update with no argument change.
// ModifyPlan plans an update for a PENDING connection so this apply waits for it.
func (r *privateConnectionResource) applyUnchanged(ctx context.Context, resp *resource.UpdateResponse, state privateConnectionModel, id uuid.UUID) {
	if isPending(state) {
		r.waitActive(ctx, resp, state, id)

		return
	}

	r.refresh(ctx, resp, state, id)
}

// waitActive waits for a PENDING connection that has no configuration change.
func (r *privateConnectionResource) waitActive(ctx context.Context, resp *resource.UpdateResponse, state privateConnectionModel, id uuid.UUID) {
	conn, werr := wait(ctx, r.ClientWithResponsesInterface, id, config.PrivateConnectionCreationTimeout, waitUntilActive,
		waitConditionStatus(management.ClusterPrivateConnectionStatusACTIVE))
	if werr != nil {
		r.keepPriorState(ctx, resp, state, werr.Summary, werr.Detail)

		return
	}

	result := toPrivateConnectionModel(ctx, conn)
	fillOmitted(ctx, &result, state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &result)...)
}

func (r *privateConnectionResource) refresh(ctx context.Context, resp *resource.UpdateResponse, state privateConnectionModel, id uuid.UUID) {
	conn, notFound, err := readConnection(ctx, r.ClientWithResponsesInterface, id)
	if err != nil || notFound || util.Deref(conn.Status) == management.ClusterPrivateConnectionStatusDELETED {
		detail := "The private connection is no longer available."
		if err != nil {
			detail = err.Detail
		}
		r.keepPriorState(ctx, resp, state, "Failed to refresh private connection", detail)

		return
	}

	result := toPrivateConnectionModel(ctx, *conn)
	fillOmitted(ctx, &result, state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &result)...)
}

func createRequest(ctx context.Context, plan privateConnectionModel, clusterID uuid.UUID, connectionType management.PrivateConnectionCreateV2Type) management.PrivateConnectionCreateV2 {
	body := management.PrivateConnectionCreateV2{
		AllowList:     util.MaybeString(plan.AllowList),
		ClusterID:     clusterID,
		KaiEndpointID: util.MaybeString(plan.KaiEndpointID),
		ServiceName:   util.MaybeString(plan.ServiceName),
		Type:          &connectionType,
	}
	// The API rejects a port when custom sockets are not enabled, so the defaults stay out of the request.
	if plan.SQLPort.ValueFloat32() != defaultSQLPort {
		body.SqlPort = util.MaybeFloat32(plan.SQLPort)
	}
	if plan.WebsocketsPort.ValueFloat32() != defaultWebsocketPort {
		body.WebsocketsPort = util.MaybeFloat32(plan.WebsocketsPort)
	}
	if regions, unknown := regionStrings(ctx, plan.SupportedCrossRegions); !unknown && len(regions) > 0 {
		body.SupportedCrossRegions = &regions
	}

	return body
}

func updateRequest(ctx context.Context, state, plan privateConnectionModel) (management.PrivateConnectionUpdateV2, bool) {
	body := management.PrivateConnectionUpdateV2{}
	changed := false
	if !plan.AllowList.Equal(state.AllowList) {
		body.AllowList = util.MaybeString(plan.AllowList)
		changed = true
	}
	if !plan.SupportedCrossRegions.Equal(state.SupportedCrossRegions) {
		regions, _ := regionStrings(ctx, plan.SupportedCrossRegions)
		if regions == nil {
			regions = []string{}
		}
		body.SupportedCrossRegions = &regions
		changed = true
	}

	return body, changed
}

func createConditions(ctx context.Context, plan privateConnectionModel) []waitCondition {
	conditions := []waitCondition{waitConditionStatus(management.ClusterPrivateConnectionStatusACTIVE)}
	if util.IsConfiguredString(plan.AllowList) {
		conditions = append(conditions, waitConditionAllowList(plan.AllowList.ValueString()))
	}
	if regions, unknown := regionStrings(ctx, plan.SupportedCrossRegions); !unknown && len(regions) > 0 {
		conditions = append(conditions, waitConditionRegions(regions))
	}

	return conditions
}

func updateConditions(ctx context.Context, state, plan privateConnectionModel) []waitCondition {
	conditions := []waitCondition{waitConditionStatus(management.ClusterPrivateConnectionStatusACTIVE)}
	if !plan.AllowList.Equal(state.AllowList) && util.IsConfiguredString(plan.AllowList) {
		conditions = append(conditions, waitConditionAllowList(plan.AllowList.ValueString()))
	}
	if !plan.SupportedCrossRegions.Equal(state.SupportedCrossRegions) {
		regions, _ := regionStrings(ctx, plan.SupportedCrossRegions)
		conditions = append(conditions, waitConditionRegions(regions))
	}

	return conditions
}

func createPlan(ctx context.Context, req resource.CreateRequest, diags *diag.Diagnostics) (privateConnectionModel, bool) {
	var plan privateConnectionModel
	diags.Append(req.Plan.Get(ctx, &plan)...)
	if diags.HasError() {
		return plan, false
	}
	if err := configuredServiceNameError(ctx, req.Config, plan, diags); err != nil || diags.HasError() {
		if err != nil {
			diags.AddError(err.Summary, err.Detail)
		}

		return plan, false
	}
	if err := validateConfiguration(plan, true); err != nil {
		diags.AddError(err.Summary, err.Detail)

		return plan, false
	}

	return plan, true
}

func configuredServiceNameError(ctx context.Context, config tfsdk.Config, plan privateConnectionModel, diags *diag.Diagnostics) *util.SummaryWithDetailError {
	var configured privateConnectionModel
	readDiags := config.Get(ctx, &configured)
	diags.Append(readDiags...)
	if readDiags.HasError() {
		return nil
	}

	return validateServiceNameConfig(configured, plan)
}

func parseUUID(value, what string) (uuid.UUID, *util.SummaryWithDetailError) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.UUID{}, &util.SummaryWithDetailError{
			Summary: fmt.Sprintf("Invalid %s", what),
			Detail:  fmt.Sprintf("The %s %q is not a valid UUID: %s", what, value, err),
		}
	}

	return id, nil
}
