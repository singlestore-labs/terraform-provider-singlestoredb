package sql

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const ResourceName = "sql_execute"

var (
	_ resource.Resource                = &sqlExecuteResource{}
	_ resource.ResourceWithModifyPlan  = &sqlExecuteResource{}
	_ resource.ResourceWithImportState = &sqlExecuteResource{}
)

type sqlExecuteResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Endpoint     types.String `tfsdk:"endpoint"`
	Username     types.String `tfsdk:"username"`
	Password     types.String `tfsdk:"password"`
	Database     types.String `tfsdk:"database"`
	Execute      types.String `tfsdk:"execute"`
	ExecuteArgs  types.List   `tfsdk:"execute_args"`
	Revert       types.String `tfsdk:"revert"`
	Query        types.String `tfsdk:"query"`
	QueryArgs    types.List   `tfsdk:"query_args"`
	QueryResults types.List   `tfsdk:"query_results"`
	LastInsertID types.Int64  `tfsdk:"last_insert_id"`
	RowsAffected types.Int64  `tfsdk:"rows_affected"`
}

type sqlExecuteResource struct{}

func NewResource() resource.Resource {
	return &sqlExecuteResource{}
}

func (r *sqlExecuteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, ResourceName)
}

func (r *sqlExecuteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Execute SQL statements against a SingleStore Helios workspace via the Data API. " +
			"Use for DDL and DML with optional read-back for drift detection. " +
			"Requires HTTPS access to the workspace host on port 443.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Random UUID assigned at create time.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"endpoint": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Workspace SQL endpoint (host or host:port). Typically `singlestoredb_workspace.<n>.endpoint`. The provider strips any port and uses HTTPS on port 443.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"username": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "SQL user name, or `*` when using JWT authentication.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: fmt.Sprintf("SQL user password or JWT when `username` is `*`. Falls back to `%s` when unset.", config.EnvSQLUserPassword),
			},
			"database": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Context database for execute, revert, and query.",
			},
			"execute": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "SQL statement run on create. Changing this value forces replacement.",
			},
			"execute_args": schema.ListAttribute{
				Optional:            true,
				Sensitive:           true,
				ElementType:         types.StringType,
				MarkdownDescription: "Positional arguments for `?` placeholders in `execute`.",
			},
			"revert": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "SQL statement run on destroy. Required so destroy is meaningful.",
			},
			"query": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional read-back SQL. Re-executed on every read; results exposed as `query_results`.",
			},
			"query_args": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Positional arguments for `?` placeholders in `query`.",
			},
			"query_results": schema.ListAttribute{
				Computed:            true,
				ElementType:         QueryResultsElementType,
				MarkdownDescription: "Rows from the first result set of `query`. All values are strings. Empty when `query` is unset or fails.",
			},
			"last_insert_id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Last insert ID from the Data API exec response (0 when not applicable).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"rows_affected": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Rows affected from the Data API exec response.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *sqlExecuteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sqlExecuteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	password, serr := resolvePassword(plan.Password)
	if serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	client, serr := buildClient(plan.Endpoint.ValueString(), plan.Username.ValueString(), password)
	if serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	executeArgs, diags := ListStrings(ctx, plan.ExecuteArgs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	execResp, err := client.Exec(ctx, ExecRequest{
		SQL:      plan.Execute.ValueString(),
		Args:     StringArgsToAny(executeArgs),
		Database: plan.Database.ValueString(),
	})
	if err != nil {
		serr := DiagnosticFromError(err)
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	plan.ID = types.StringValue(uuid.NewString())
	plan.LastInsertID = types.Int64Value(execResp.LastInsertID)
	plan.RowsAffected = types.Int64Value(execResp.RowsAffected)
	plan.Password = passwordForState(plan.Password)

	queryResults, readDiags := r.readQueryFromModel(ctx, client, plan, queryFailureIsError)
	resp.Diagnostics.Append(readDiags...)
	plan.QueryResults = queryResults

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sqlExecuteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sqlExecuteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.Query.IsNull() || state.Query.ValueString() == "" {
		state.QueryResults = EmptyQueryResults()
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

		return
	}

	password, serr := resolvePassword(state.Password)
	if serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	client, serr := buildClient(state.Endpoint.ValueString(), state.Username.ValueString(), password)
	if serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	queryResults, readDiags := r.readQueryFromModel(ctx, client, state, queryFailureIsWarning)
	resp.Diagnostics.Append(readDiags...)
	state.QueryResults = queryResults

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sqlExecuteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sqlExecuteResourceModel
	var state sqlExecuteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if passwordConfiguredInPlan(plan.Password) {
		state.Password = plan.Password
	}

	state.Database = plan.Database
	state.Revert = plan.Revert
	state.Query = plan.Query
	state.QueryArgs = plan.QueryArgs

	password, serr := resolvePassword(state.Password)
	if serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	client, serr := buildClient(state.Endpoint.ValueString(), state.Username.ValueString(), password)
	if serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	queryResults, readDiags := r.readQueryFromModel(ctx, client, state, queryFailureIsError)
	resp.Diagnostics.Append(readDiags...)
	state.QueryResults = queryResults

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sqlExecuteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sqlExecuteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	password, serr := resolvePassword(state.Password)
	if serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	client, serr := buildClient(state.Endpoint.ValueString(), state.Username.ValueString(), password)
	if serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	_, err := client.Exec(ctx, ExecRequest{
		SQL:      state.Revert.ValueString(),
		Database: state.Database.ValueString(),
	})
	if err != nil {
		// Deliberate trade-off: when the workspace is unreachable we let destroy
		// succeed so a deleted/suspended workspace does not wedge `terraform destroy`.
		// IsUnreachable also matches transient network failures, so a blip here can
		// drop the resource from state without running revert. Surfacing it would
		// block destroy of a genuinely-gone workspace, which is the worse failure.
		if IsUnreachable(err) {
			return
		}

		serr := DiagnosticFromError(err)
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}
}

func (r *sqlExecuteResource) ImportState(_ context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError(
		"Import not supported",
		"SQL statements are not addressable; import is not supported for singlestoredb_sql_execute.",
	)
}

func (r *sqlExecuteResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var plan, state sqlExecuteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.RequiresReplace = append(resp.RequiresReplace, modifyPlanExecuteReplacement(ctx, plan, state, &resp.Diagnostics)...)

	if modifyPlanQueryChanged(ctx, plan, state) {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("query_results"), types.ListUnknown(QueryResultsElementType))...)
	}
}

func modifyPlanExecuteReplacement(ctx context.Context, plan, state sqlExecuteResourceModel, diags *diag.Diagnostics) []path.Path {
	executeChanged := !state.Execute.IsNull() && state.Execute.ValueString() != "" &&
		plan.Execute.ValueString() != state.Execute.ValueString()
	executeArgsChanged := executeArgsDiffer(ctx, state.ExecuteArgs, plan.ExecuteArgs)
	if !executeChanged && !executeArgsChanged {
		return nil
	}

	diags.AddError(
		"Execute statement change requires replacement",
		"Changing execute or execute_args forces resource replacement. Update revert accordingly.",
	)

	requiresReplace := []path.Path{path.Root("execute")}
	if executeArgsChanged {
		requiresReplace = append(requiresReplace, path.Root("execute_args"))
	}

	return requiresReplace
}

func modifyPlanQueryChanged(ctx context.Context, plan, state sqlExecuteResourceModel) bool {
	return !plan.Query.Equal(state.Query) ||
		executeArgsDiffer(ctx, state.QueryArgs, plan.QueryArgs)
}

// queryFailureMode controls how a failed read-back query surfaces. On create and
// update a broken query is a configuration error; on read it is treated as drift
// (a warning) so refresh does not hard-fail when the workspace is reachable.
type queryFailureMode bool

const (
	queryFailureIsError   queryFailureMode = false
	queryFailureIsWarning queryFailureMode = true
)

func (r *sqlExecuteResource) readQueryFromModel(ctx context.Context, client *Client, model sqlExecuteResourceModel, onFailure queryFailureMode) (types.List, diag.Diagnostics) {
	if model.Query.IsNull() || model.Query.ValueString() == "" {
		return EmptyQueryResults(), nil
	}

	queryArgs, diags := ListStrings(ctx, model.QueryArgs)
	if diags.HasError() {
		return EmptyQueryResults(), diags
	}

	return readQuery(ctx, client, model.Database.ValueString(), model.Query.ValueString(), StringArgsToAny(queryArgs), onFailure)
}

func readQuery(ctx context.Context, client *Client, database, query string, queryArgs []any, onFailure queryFailureMode) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	resp, err := client.QueryRows(ctx, ExecRequest{
		SQL:      query,
		Args:     queryArgs,
		Database: database,
	})
	if err != nil {
		serr := DiagnosticFromError(err)
		if onFailure == queryFailureIsWarning {
			diags.AddWarning(serr.Summary, serr.Detail)
		} else {
			diags.AddError(serr.Summary, serr.Detail)
		}

		return EmptyQueryResults(), diags
	}

	stringified, err := StringifyRows(firstResultSetRows(resp))
	if err != nil {
		diags.AddError("Failed to parse query results", err.Error())

		return EmptyQueryResults(), diags
	}

	list, listDiags := RowsToTFList(ctx, stringified)
	diags.Append(listDiags...)

	return list, diags
}

func buildClient(endpoint, username, password string) (*Client, *util.SummaryWithDetailError) {
	baseURL, err := DataAPIURL(endpoint)
	if err != nil {
		return nil, InvalidEndpointDiagnostic(err)
	}

	return NewClient(baseURL, username, password), nil
}

func executeArgsDiffer(ctx context.Context, a, b types.List) bool {
	as, aDiags := ListStrings(ctx, a)
	bs, bDiags := ListStrings(ctx, b)
	if aDiags.HasError() || bDiags.HasError() {
		return !a.Equal(b)
	}

	if len(as) == 0 && len(bs) == 0 {
		return false
	}

	return !slices.Equal(as, bs)
}
