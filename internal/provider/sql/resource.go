package sql

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	ResourceName = "sql"
)

var (
	_ resource.Resource                = &sqlResource{}
	_ resource.ResourceWithImportState = &sqlResource{}
	_ resource.ResourceWithModifyPlan  = &sqlResource{}
)

type sqlExecutor interface {
	Exec(ctx context.Context, query string) error
	Query(ctx context.Context, query string) ([]map[string]string, error)
	Close() error
}

type sqlResource struct {
	openClient func(context.Context, ConnectionConfig) (sqlExecutor, error)
}

type sqlResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Endpoint     types.String `tfsdk:"endpoint"`
	Username     types.String `tfsdk:"username"`
	Password     types.String `tfsdk:"password"`
	Database     types.String `tfsdk:"database"`
	Port         types.Int64  `tfsdk:"port"`
	TLS          types.String `tfsdk:"tls"`
	Execute      types.String `tfsdk:"execute"`
	Revert       types.String `tfsdk:"revert"`
	Query        types.String `tfsdk:"query"`
	QueryResults types.List   `tfsdk:"query_results"`
}

// NewResource is a helper function to simplify the provider implementation.
func NewResource() resource.Resource {
	return &sqlResource{
		openClient: func(ctx context.Context, cfg ConnectionConfig) (sqlExecutor, error) {
			return Open(ctx, cfg)
		},
	}
}

func (r *sqlResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, ResourceName)
}

func (r *sqlResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Execute SQL statements against a SingleStore workspace. This resource is useful for bootstrapping schemas, users, and grants after workspace provisioning.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the SQL execution resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"endpoint": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The workspace SQL endpoint hostname.",
			},
			"username": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The SQL username used to connect to the workspace.",
				Default:             stringdefault.StaticString(defaultUsername),
			},
			"password": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "The SQL user password used to connect to the workspace.",
			},
			"database": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The default database to connect to.",
			},
			"port": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The SQL port used to connect to the workspace.",
				Default:             int64default.StaticInt64(defaultPort),
			},
			"tls": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "TLS mode for the SQL connection. Valid values are `true`, `false`, `skip-verify`, and `preferred`.",
				Default:             stringdefault.StaticString(defaultTLS),
			},
			"execute": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "SQL statement to execute when the resource is created. Changing this value forces recreation of the resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"revert": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "SQL statement to execute when the resource is destroyed.",
			},
			"query": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional SQL statement to run on every read. Use this to verify the state of objects created by `execute`.",
			},
			"query_results": schema.ListAttribute{
				Computed:            true,
				MarkdownDescription: "List of key-value maps retrieved after executing the optional `query` statement.",
				ElementType: types.MapType{
					ElemType: types.StringType,
				},
			},
		},
	}
}

func (r *sqlResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sqlResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := r.openClient(ctx, connectionConfigFromModel(plan))
	if err != nil {
		resp.Diagnostics.AddError("Unable to connect to SingleStore workspace", err.Error())

		return
	}
	defer client.Close()

	tflog.Trace(ctx, "executing SQL statement")
	if err := client.Exec(ctx, plan.Execute.ValueString()); err != nil {
		resp.Diagnostics.AddError("SQL execute statement failed", err.Error())

		return
	}

	id, err := uuid.NewRandom()
	if err != nil {
		resp.Diagnostics.AddError("Unable to generate resource ID", err.Error())

		return
	}

	plan.ID = types.StringValue(id.String())
	queryResults, queryDiags := r.readQueryResults(ctx, client, plan.Query)
	resp.Diagnostics.Append(queryDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.QueryResults = queryResults
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *sqlResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sqlResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := r.openClient(ctx, connectionConfigFromModel(state))
	if err != nil {
		resp.Diagnostics.AddError("Unable to connect to SingleStore workspace", err.Error())

		return
	}
	defer client.Close()

	queryResults, queryDiags := r.readQueryResults(ctx, client, state.Query)
	resp.Diagnostics.Append(queryDiags...)
	state.QueryResults = queryResults

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *sqlResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sqlResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state sqlResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Query.Equal(state.Query) {
		diags = resp.State.Set(ctx, &plan)
		resp.Diagnostics.Append(diags...)

		return
	}

	client, err := r.openClient(ctx, connectionConfigFromModel(plan))
	if err != nil {
		resp.Diagnostics.AddError("Unable to connect to SingleStore workspace", err.Error())

		return
	}
	defer client.Close()

	queryResults, queryDiags := r.readQueryResults(ctx, client, plan.Query)
	resp.Diagnostics.Append(queryDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.QueryResults = queryResults
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *sqlResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sqlResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	revert := strings.TrimSpace(state.Revert.ValueString())
	if revert == "" {
		return
	}

	client, err := r.openClient(ctx, connectionConfigFromModel(state))
	if err != nil {
		resp.Diagnostics.AddError("Unable to connect to SingleStore workspace", err.Error())

		return
	}
	defer client.Close()

	tflog.Trace(ctx, "reverting SQL statement")
	if err := client.Exec(ctx, revert); err != nil {
		resp.Diagnostics.AddError("SQL revert statement failed", err.Error())
	}
}

func (r *sqlResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan sqlResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state *sqlResourceModel
	if !req.State.Raw.IsNull() {
		state = &sqlResourceModel{}
		diags = req.State.Get(ctx, state)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	if state == nil || !plan.Query.Equal(state.Query) {
		resp.Plan.SetAttribute(ctx, path.Root("query_results"), types.ListUnknown(types.MapType{
			ElemType: types.StringType,
		}))
	}
}

func (r *sqlResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if _, err := uuid.Parse(req.ID); err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"The provided import ID is not a valid UUID: \""+req.ID+"\".",
		)

		return
	}

	resource.ImportStatePassthroughID(ctx, path.Root(config.IDAttribute), req, resp)
}

func (r *sqlResource) readQueryResults(ctx context.Context, client sqlExecutor, query types.String) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	if query.IsNull() || query.IsUnknown() || strings.TrimSpace(query.ValueString()) == "" {
		return types.ListNull(types.MapType{ElemType: types.StringType}), diags
	}

	rows, err := client.Query(ctx, query.ValueString())
	if err != nil {
		diags.AddWarning(
			"SQL query statement failed",
			err.Error(),
		)

		return types.ListNull(types.MapType{ElemType: types.StringType}), diags
	}

	results, convDiags := rowsToList(rows)
	diags.Append(convDiags...)

	return results, diags
}

func rowsToList(rows []map[string]string) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	if len(rows) == 0 {
		return types.ListValueMust(types.MapType{ElemType: types.StringType}, []attr.Value{}), diags
	}

	elements := make([]attr.Value, 0, len(rows))
	for _, row := range rows {
		values := make(map[string]attr.Value, len(row))
		for key, value := range row {
			values[key] = basetypes.NewStringValue(value)
		}

		rowValue, rowDiags := types.MapValue(types.StringType, values)
		diags.Append(rowDiags...)
		if diags.HasError() {
			return types.ListNull(types.MapType{ElemType: types.StringType}), diags
		}

		elements = append(elements, rowValue)
	}

	return types.ListValueMust(types.MapType{ElemType: types.StringType}, elements), diags
}

func connectionConfigFromModel(model sqlResourceModel) ConnectionConfig {
	return ConnectionConfig{
		Endpoint: model.Endpoint.ValueString(),
		Username: model.Username.ValueString(),
		Password: model.Password.ValueString(),
		Database: model.Database.ValueString(),
		Port:     int(model.Port.ValueInt64()),
		TLS:      model.TLS.ValueString(),
	}
}
