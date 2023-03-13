package workspacegroups

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/util"
)

const (
	dataSourceListName = "workspace_groups"
)

// workspaceGroupsDataSourceList is the data source implementation.
type workspaceGroupsDataSourceList struct {
	management.ClientWithResponsesInterface
}

// workspaceGroupsListDataSourceModel maps the data source schema data.
type workspaceGroupsListDataSourceModel struct {
	ID              types.String                    `tfsdk:"id"`
	WorkspaceGroups []workspaceGroupDataSourceModel `tfsdk:"workspace_groups"`
}

type updateWindowDataSourceModel struct {
	Hour types.Int64 `tfsdk:"hour"`
	Day  types.Int64 `tfsdk:"day"`
}

var _ datasource.DataSourceWithConfigure = &workspaceGroupsDataSourceList{}

// NewDataSourceList is a helper function to simplify the provider implementation.
func NewDataSourceList() datasource.DataSource {
	return &workspaceGroupsDataSourceList{}
}

// Metadata returns the data source type name.
func (d *workspaceGroupsDataSourceList) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, dataSourceListName)
}

// Schema defines the schema for the data source.
func (d *workspaceGroupsDataSourceList) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			config.TestIDAttribute: schema.StringAttribute{
				Computed: true,
			},
			dataSourceListName: schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: newWorkspaceGroupDataSourceSchemaAttributes(workspaceGroupDataSourceSchemaConfig{
						computeWorkspaceGroupID: true,
					}),
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *workspaceGroupsDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	workspaceGroups, err := d.GetV1WorkspaceGroupsWithResponse(ctx, &management.GetV1WorkspaceGroupsParams{})
	if err != nil {
		resp.Diagnostics.AddError(
			"SingleStore API client failed to list workspace groups",
			"An unexpected error occurred when calling SingleStore API workspace groups. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"SingleStore client error: "+err.Error(),
		)

		return
	}

	code := workspaceGroups.StatusCode()
	if code != http.StatusOK {
		resp.Diagnostics.AddError(
			fmt.Sprintf("SingleStore API client returned status code %s while listing workspace groups", http.StatusText(code)),
			"An unsuccessful status code occurred when calling SingleStore API workspace groups. "+
				fmt.Sprintf("Make sure to set the %s value in the configuration or use the %s environment variable. ", config.APIKeyAttribute, config.EnvAPIKey)+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"SingleStore client response body: "+string(workspaceGroups.Body),
		)

		return
	}

	result := workspaceGroupsListDataSourceModel{
		ID: types.StringValue(config.TestIDValue),
	}

	for _, workspaceGroup := range util.Deref(workspaceGroups.JSON200) {
		result.WorkspaceGroups = append(result.WorkspaceGroups, toWorkspaceGroupDataSourceModel(workspaceGroup))
	}

	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *workspaceGroupsDataSourceList) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func toWorkspaceGroupDataSourceModel(workspaceGroup management.WorkspaceGroup) workspaceGroupDataSourceModel {
	return workspaceGroupDataSourceModel{
		ID:               types.StringValue(config.TestIDValue),
		Name:             types.StringValue(workspaceGroup.Name),
		State:            util.WorkspaceGroupStateStringValue(workspaceGroup.State),
		WorkspaceGroupID: util.UUIDStringValue(workspaceGroup.WorkspaceGroupID),
		FirewallRanges:   util.FirewallRanges(workspaceGroup.FirewallRanges),
		AllowAllTraffic:  util.MaybeBoolValue(workspaceGroup.AllowAllTraffic),
		CreatedAt:        types.StringValue(workspaceGroup.CreatedAt),
		ExpiresAt:        util.MaybeStringValue(workspaceGroup.ExpiresAt),
		RegionID:         util.UUIDStringValue(workspaceGroup.RegionID),
		UpdateWindow:     toUpdateWindowDataSourceModel(workspaceGroup.UpdateWindow),
	}
}

func toUpdateWindowDataSourceModel(uw *management.UpdateWindow) *updateWindowDataSourceModel {
	if uw == nil {
		return nil
	}

	return &updateWindowDataSourceModel{
		Hour: types.Int64Value(int64(uw.Hour)),
		Day:  types.Int64Value(int64(uw.Day)),
	}
}
