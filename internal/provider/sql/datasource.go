package sql

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const DataSourceName = "sql_query"

var _ datasource.DataSource = &sqlQueryDataSource{}

type sqlQueryDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Endpoint types.String `tfsdk:"endpoint"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	Database types.String `tfsdk:"database"`
	Query    types.String `tfsdk:"query"`
	Args     types.List   `tfsdk:"args"`
	Rows     types.List   `tfsdk:"rows"`
}

type sqlQueryDataSource struct{}

func NewDataSourceQuery() datasource.DataSource {
	return &sqlQueryDataSource{}
}

func (d *sqlQueryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceName)
}

func (d *sqlQueryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Run a read-only SQL query against a SingleStore Helios workspace via the Data API. " +
			"Re-runs on every plan and apply; heavy queries add latency and load to the workspace. " +
			"Use singlestoredb_sql_execute for DDL and DML.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Hash of endpoint, query, and args so plan diffs when inputs change.",
			},
			"endpoint": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Workspace SQL endpoint (host or host:port). Typically `singlestoredb_workspace.<n>.endpoint`. The provider strips any port and uses HTTPS on port 443.",
			},
			"username": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "SQL user name, or `*` when using JWT authentication.",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: fmt.Sprintf("SQL user password or JWT when `username` is `*`. Falls back to `%s` when unset.", config.EnvSQLUserPassword),
			},
			"database": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Context database for the query.",
			},
			"query": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Read-only SQL (typically SELECT). Only the first result set is returned; all cell values are strings.",
			},
			"args": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Positional arguments for `?` placeholders in `query`.",
			},
			"rows": schema.ListAttribute{
				Computed:            true,
				ElementType:         QueryResultsElementType,
				MarkdownDescription: "Rows from the first result set. All values are strings.",
			},
		},
	}
}

func (d *sqlQueryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model sqlQueryDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	password, serr := resolvePassword(model.Password)
	if serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	client, serr := buildClient(model.Endpoint.ValueString(), model.Username.ValueString(), password)
	if serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	args, diags := ListStrings(ctx, model.Args)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	queryResp, err := client.QueryRows(ctx, ExecRequest{
		SQL:      model.Query.ValueString(),
		Args:     StringArgsToAny(args),
		Database: model.Database.ValueString(),
	})
	if err != nil {
		serr := DiagnosticFromError(err)
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	stringified, err := StringifyRows(firstResultSetRows(queryResp))
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse query results", err.Error())

		return
	}

	rows, listDiags := RowsToTFList(ctx, stringified)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	model.Rows = rows
	model.ID = types.StringValue(queryDataSourceID(model.Endpoint.ValueString(), model.Query.ValueString(), args))
	model.Password = passwordForState(model.Password)

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func queryDataSourceID(endpoint, query string, args []string) string {
	argsJSON, err := json.Marshal(args)
	if err != nil {
		argsJSON = []byte("null")
	}

	sum := sha256.Sum256([]byte(endpoint + "\x00" + query + "\x00" + string(argsJSON)))

	return hex.EncodeToString(sum[:])
}
