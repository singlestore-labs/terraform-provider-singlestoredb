package workspaces

import (
	"context"

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
	DataSourceIdentityName                      = "workspace_identity"
	DataSourcePrivateConnectionsName            = "workspace_private_connections"
	DataSourcePrivateConnectionsKaiName         = "workspace_private_connections_kai"
	DataSourceOutboundAllowListName             = "workspace_private_connections_outbound_allow_list"
	defaultWorkspaceSQLPort             float32 = 3306
	defaultWorkspaceWebsocketPort       float32 = 443
)

type workspaceIdentityDataSource struct {
	management.ClientWithResponsesInterface
}

type workspacePrivateConnectionsDataSource struct {
	management.ClientWithResponsesInterface
}

type workspacePrivateConnectionsKaiDataSource struct {
	management.ClientWithResponsesInterface
}

type workspacePrivateConnectionsOutboundAllowListDataSource struct {
	management.ClientWithResponsesInterface
}

type workspaceIdentityDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Identity    types.String `tfsdk:"identity"`
}

type workspacePrivateConnectionsDataSourceModel struct {
	ID                 types.String                      `tfsdk:"id"`
	WorkspaceID        types.String                      `tfsdk:"workspace_id"`
	PrivateConnections []workspacePrivateConnectionModel `tfsdk:"private_connections"`
}

type workspacePrivateConnectionModel struct {
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

type workspacePrivateConnectionsKaiDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	ServiceName types.String `tfsdk:"service_name"`
}

type workspacePrivateConnectionsOutboundAllowListDataSourceModel struct {
	ID                types.String                          `tfsdk:"id"`
	WorkspaceID       types.String                          `tfsdk:"workspace_id"`
	OutboundAllowList []workspaceOutboundAllowListItemModel `tfsdk:"outbound_allow_list"`
}

type workspaceOutboundAllowListItemModel struct {
	OutboundAllowList types.String `tfsdk:"outbound_allow_list"`
}

var (
	_ datasource.DataSourceWithConfigure = &workspaceIdentityDataSource{}
	_ datasource.DataSourceWithConfigure = &workspacePrivateConnectionsDataSource{}
	_ datasource.DataSourceWithConfigure = &workspacePrivateConnectionsKaiDataSource{}
	_ datasource.DataSourceWithConfigure = &workspacePrivateConnectionsOutboundAllowListDataSource{}
)

func NewDataSourceIdentity() datasource.DataSource {
	return &workspaceIdentityDataSource{}
}

func NewDataSourcePrivateConnections() datasource.DataSource {
	return &workspacePrivateConnectionsDataSource{}
}

func NewDataSourcePrivateConnectionsKai() datasource.DataSource {
	return &workspacePrivateConnectionsKaiDataSource{}
}

func NewDataSourcePrivateConnectionsOutboundAllowList() datasource.DataSource {
	return &workspacePrivateConnectionsOutboundAllowListDataSource{}
}

func (d *workspaceIdentityDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceIdentityName)
}

func (d *workspacePrivateConnectionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourcePrivateConnectionsName)
}

func (d *workspacePrivateConnectionsKaiDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourcePrivateConnectionsKaiName)
}

func (d *workspacePrivateConnectionsOutboundAllowListDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceOutboundAllowListName)
}

func (d *workspaceIdentityDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieve cloud workload identity for a specific workspace.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed: true,
			},
			"workspace_id": workspaceIDSchemaAttribute(),
			"identity": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The cloud workload identity bound to the workspace.",
			},
		},
	}
}

func (d *workspacePrivateConnectionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List private connections attached to a specific workspace.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed: true,
			},
			"workspace_id": workspaceIDSchemaAttribute(),
			"private_connections": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Private connections attached to the workspace.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: workspacePrivateConnectionSchemaAttributes(),
				},
			},
		},
	}
}

func (d *workspacePrivateConnectionsKaiDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieve private connection Kai service details for a specific workspace.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed: true,
			},
			"workspace_id": workspaceIDSchemaAttribute(),
			"service_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "VPC endpoint service name for Kai.",
			},
		},
	}
}

func (d *workspacePrivateConnectionsOutboundAllowListDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List outbound allow list entries configured for private connections in a workspace.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed: true,
			},
			"workspace_id": workspaceIDSchemaAttribute(),
			"outbound_allow_list": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Outbound allow list entries associated with workspace private connections.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"outbound_allow_list": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The account ID allowed for outbound connections.",
						},
					},
				},
			},
		},
	}
}

func (d *workspaceIdentityDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data workspaceIdentityDataSourceModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID, ok := parseWorkspaceID(data.WorkspaceID, resp)
	if !ok {
		return
	}

	identityResp, err := d.GetV1WorkspacesWorkspaceIDIdentityWithResponse(ctx, workspaceID)
	if serr := util.StatusOK(identityResp, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	result := workspaceIdentityDataSourceModel{
		ID:          types.StringValue(config.TestIDValue),
		WorkspaceID: data.WorkspaceID,
		Identity:    types.StringValue(identityResp.JSON200.Identity),
	}
	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (d *workspacePrivateConnectionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data workspacePrivateConnectionsDataSourceModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID, ok := parseWorkspaceID(data.WorkspaceID, resp)
	if !ok {
		return
	}

	privateConnectionsResp, err := d.GetV1WorkspacesWorkspaceIDPrivateConnectionsWithResponse(
		ctx,
		workspaceID,
		&management.GetV1WorkspacesWorkspaceIDPrivateConnectionsParams{},
	)
	if serr := util.StatusOK(privateConnectionsResp, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	privateConnections, merr := util.MapWithError(util.Deref(privateConnectionsResp.JSON200), toWorkspacePrivateConnectionModel)
	if merr != nil {
		resp.Diagnostics.AddError(merr.Summary, merr.Detail)

		return
	}

	result := workspacePrivateConnectionsDataSourceModel{
		ID:                 types.StringValue(config.TestIDValue),
		WorkspaceID:        data.WorkspaceID,
		PrivateConnections: privateConnections,
	}
	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (d *workspacePrivateConnectionsKaiDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data workspacePrivateConnectionsKaiDataSourceModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID, ok := parseWorkspaceID(data.WorkspaceID, resp)
	if !ok {
		return
	}

	kaiResp, err := d.GetV1WorkspacesWorkspaceIDPrivateConnectionsKaiWithResponse(ctx, workspaceID)
	if serr := util.StatusOK(kaiResp, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	result := workspacePrivateConnectionsKaiDataSourceModel{
		ID:          types.StringValue(config.TestIDValue),
		WorkspaceID: data.WorkspaceID,
		ServiceName: util.MaybeStringValue(kaiResp.JSON200.ServiceName),
	}
	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (d *workspacePrivateConnectionsOutboundAllowListDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data workspacePrivateConnectionsOutboundAllowListDataSourceModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID, ok := parseWorkspaceID(data.WorkspaceID, resp)
	if !ok {
		return
	}

	outboundAllowListResp, err := d.GetV1WorkspacesWorkspaceIDPrivateConnectionsOutboundAllowListWithResponse(ctx, workspaceID)
	if serr := util.StatusOK(outboundAllowListResp, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	outboundAllowList := util.Map(
		util.Deref(outboundAllowListResp.JSON200),
		func(item management.PrivateConnectionOutboundAllowList) workspaceOutboundAllowListItemModel {
			return workspaceOutboundAllowListItemModel{
				OutboundAllowList: util.MaybeStringValue(item.OutboundAllowList),
			}
		},
	)

	result := workspacePrivateConnectionsOutboundAllowListDataSourceModel{
		ID:                types.StringValue(config.TestIDValue),
		WorkspaceID:       data.WorkspaceID,
		OutboundAllowList: outboundAllowList,
	}
	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (d *workspaceIdentityDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func (d *workspacePrivateConnectionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func (d *workspacePrivateConnectionsKaiDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func (d *workspacePrivateConnectionsOutboundAllowListDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func workspaceIDSchemaAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Required:            true,
		MarkdownDescription: "The unique identifier of the workspace.",
		Validators:          []validator.String{util.NewUUIDValidator()},
	}
}

func parseWorkspaceID(workspaceID types.String, resp *datasource.ReadResponse) (uuid.UUID, bool) {
	id, err := uuid.Parse(workspaceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("workspace_id"),
			"Invalid workspace ID",
			"The workspace ID should be a valid UUID",
		)

		return uuid.Nil, false
	}

	return id, true
}

func workspacePrivateConnectionSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		config.IDAttribute: schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The unique identifier of the private connection.",
		},
		"active_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The timestamp of when the private connection became active.",
		},
		"allow_list": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The private connection allow list.",
		},
		"kai_endpoint_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "VPC endpoint ID for AWS.",
		},
		"created_at": schema.StringAttribute{
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
			Computed:            true,
			MarkdownDescription: "The name of the private connection service.",
		},
		"sql_port": schema.Float32Attribute{
			Computed:            true,
			MarkdownDescription: "The SQL port.",
		},
		"web_socket_port": schema.Float32Attribute{
			Computed:            true,
			MarkdownDescription: "The websockets port.",
		},
		"type": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The private connection type.",
		},
		"status": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The status of the private connection.",
		},
		"workspace_group_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The ID of the workspace group containing the private connection.",
		},
		"workspace_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The ID of the workspace to connect with.",
		},
	}
}

func toWorkspacePrivateConnectionModel(privateConnection management.PrivateConnection) (workspacePrivateConnectionModel, *util.SummaryWithDetailError) {
	var kaiEndpointID types.String
	if privateConnection.AllowedPrivateLinkIDs != nil && len(*privateConnection.AllowedPrivateLinkIDs) > 0 {
		kaiEndpointID = types.StringValue((*privateConnection.AllowedPrivateLinkIDs)[0])
	}

	model := workspacePrivateConnectionModel{
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
		model.SQLPort = types.Float32Value(defaultWorkspaceSQLPort)
	}
	if model.WebsocketsPort.IsNull() || model.WebsocketsPort.IsUnknown() {
		model.WebsocketsPort = types.Float32Value(defaultWorkspaceWebsocketPort)
	}

	return model, nil
}
