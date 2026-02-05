package flow

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
	DataSourceGetName = "flow_instance"
)

// flowInstanceDataSourceGet is the data source implementation.
type flowInstanceDataSourceGet struct {
	management.ClientWithResponsesInterface
}

// flowInstanceDataSourceModel maps flow instance schema data.
type flowInstanceDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	CreatedAt   types.String `tfsdk:"created_at"`
	DeletedAt   types.String `tfsdk:"deleted_at"`
	Endpoint    types.String `tfsdk:"endpoint"`
	Size        types.String `tfsdk:"size"`
}

type flowInstanceDataSourceSchemaConfig struct {
	computeFlowInstanceID    bool
	optionalFlowInstanceID   bool
	computedName             bool
	optionalName             bool
	flowInstanceIDValidators []validator.String
}

var _ datasource.DataSourceWithConfigure = &flowInstanceDataSourceGet{}

// NewDataSourceGet is a helper function to simplify the provider implementation.
func NewDataSourceGet() datasource.DataSource {
	return &flowInstanceDataSourceGet{}
}

// Metadata returns the data source type name.
func (d *flowInstanceDataSourceGet) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceGetName)
}

// Schema defines the schema for the data source.
func (d *flowInstanceDataSourceGet) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieve a specific Flow instance using its ID or name with this data source.",
		Attributes: newFlowInstanceDataSourceSchemaAttributes(flowInstanceDataSourceSchemaConfig{
			optionalFlowInstanceID:   true,
			optionalName:             true,
			flowInstanceIDValidators: []validator.String{util.NewUUIDValidator()},
		}),
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *flowInstanceDataSourceGet) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data flowInstanceDataSourceModel
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
func (d *flowInstanceDataSourceGet) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func newFlowInstanceDataSourceSchemaAttributes(conf flowInstanceDataSourceSchemaConfig) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		config.IDAttribute: schema.StringAttribute{
			Computed:            conf.computeFlowInstanceID,
			Optional:            conf.optionalFlowInstanceID,
			MarkdownDescription: "The unique identifier of the Flow instance. Either `id` or `name` must be specified.",
			Validators:          conf.flowInstanceIDValidators,
		},
		"name": schema.StringAttribute{
			Computed:            conf.computedName,
			Optional:            conf.optionalName,
			MarkdownDescription: "The name of the Flow instance. Either `id` or `name` must be specified.",
		},
		"workspace_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The unique identifier of the workspace associated with the Flow instance.",
		},
		"created_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The timestamp indicating when the Flow instance was initially created.",
		},
		"deleted_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The timestamp indicating when the Flow instance was terminated. If the Flow instance is active, this attribute will not be set.",
		},
		"endpoint": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The endpoint to connect to the Flow instance.",
		},
		"size": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The size of the Flow instance (in Flow size notation), such as 'F1'.",
		},
	}
}

func toFlowInstanceDataSourceModel(flow management.Flow) (flowInstanceDataSourceModel, *util.SummaryWithDetailError) {
	model := flowInstanceDataSourceModel{
		ID:          util.UUIDStringValue(flow.FlowID),
		Name:        types.StringValue(flow.Name),
		WorkspaceID: util.MaybeUUIDStringValue(flow.WorkspaceID),
		CreatedAt:   types.StringValue(flow.CreatedAt.String()),
		Endpoint:    util.MaybeStringValue(flow.Endpoint),
		Size:        util.MaybeStringValue(flow.Size),
	}

	if flow.DeletedAt != nil {
		model.DeletedAt = types.StringValue(flow.DeletedAt.String())
	} else {
		model.DeletedAt = types.StringNull()
	}

	return model, nil
}

func readByID(data flowInstanceDataSourceModel, ctx context.Context, d *flowInstanceDataSourceGet, resp *datasource.ReadResponse) {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.IDAttribute),
			"Invalid Flow instance ID",
			"The Flow instance ID should be a valid UUID",
		)

		return
	}

	flow, err := d.GetV1FlowFlowIDWithResponse(ctx, id)
	if serr := util.StatusOK(flow, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	if flow.JSON200.DeletedAt != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.IDAttribute),
			fmt.Sprintf("Flow instance with the specified ID existed, but got terminated at %s", flow.JSON200.DeletedAt.Format("2006-01-02T15:04:05.9999Z")),
			"Make sure to set the Flow instance ID of the Flow instance that exists.",
		)

		return
	}

	result, terr := toFlowInstanceDataSourceModel(*flow.JSON200)
	if terr != nil {
		resp.Diagnostics.AddError(terr.Summary, terr.Detail)

		return
	}

	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func readByName(data flowInstanceDataSourceModel, ctx context.Context, d *flowInstanceDataSourceGet, resp *datasource.ReadResponse) {
	// Get all Flow instances
	flowInstances, err := d.GetV1FlowWithResponse(ctx, &management.GetV1FlowParams{})
	if serr := util.StatusOK(flowInstances, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	var foundFlowInstances []management.Flow
	targetName := strings.TrimSpace(data.Name.ValueString())

	// Filter Flow instances by name (case-insensitive), excluding terminated ones
	for _, flow := range util.Deref(flowInstances.JSON200) {
		if strings.EqualFold(strings.TrimSpace(flow.Name), targetName) && flow.DeletedAt == nil {
			foundFlowInstances = append(foundFlowInstances, flow)
		}
	}

	if len(foundFlowInstances) == 0 {
		resp.Diagnostics.AddError(
			"Flow instance not found",
			fmt.Sprintf("No active Flow instance with the name '%s' was found. Please verify that the name is correct and that the Flow instance exists.", data.Name.ValueString()),
		)

		return
	}

	if len(foundFlowInstances) > 1 {
		resp.Diagnostics.AddError(
			"Multiple Flow instances found",
			fmt.Sprintf("Multiple Flow instances with the name '%s' were found. Please specify the Flow instance ID to uniquely identify the Flow instance.", data.Name.ValueString()),
		)

		return
	}

	result, terr := toFlowInstanceDataSourceModel(foundFlowInstances[0])
	if terr != nil {
		resp.Diagnostics.AddError(terr.Summary, terr.Detail)

		return
	}

	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}
