package workspacegroups

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
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
		MarkdownDescription: "This data source provides a list of workspace groups that the user has access to.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed: true,
			},
			dataSourceListName: schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: newWorkspaceGroupDataSourceSchemaAttributes(workspaceGroupDataSourceSchemaConfig{
						computeWorkspaceGroupID: true,
						computeName:             true,
					}),
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *workspaceGroupsDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	workspaceGroups, err := d.GetV1WorkspaceGroupsWithResponse(ctx, &management.GetV1WorkspaceGroupsParams{})
	if serr := util.StatusOK(workspaceGroups, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	projectNamesByID := map[string]string{}
	if hasWorkspaceGroupWithProjectID(util.Deref(workspaceGroups.JSON200)) {
		resolvedProjectNamesByID, perr := getProjectNamesByID(ctx, d.ClientWithResponsesInterface)
		if perr != nil {
			resp.Diagnostics.AddError(
				perr.Summary,
				perr.Detail,
			)

			return
		}
		projectNamesByID = resolvedProjectNamesByID
	}

	workspaceGroupModels := make([]workspaceGroupDataSourceModel, 0, len(util.Deref(workspaceGroups.JSON200)))
	for _, wg := range util.Deref(workspaceGroups.JSON200) {
		model, merr := toWorkspaceGroupDataSourceModel(wg, projectNamesByID)
		if merr != nil {
			resp.Diagnostics.AddError(
				merr.Summary,
				merr.Detail,
			)

			return
		}

		workspaceGroupModels = append(workspaceGroupModels, model)
	}

	result := workspaceGroupsListDataSourceModel{
		ID:              types.StringValue(config.TestIDValue),
		WorkspaceGroups: workspaceGroupModels,
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

func toWorkspaceGroupDataSourceModel(workspaceGroup management.WorkspaceGroup, projectNamesByID map[string]string) (workspaceGroupDataSourceModel, *util.SummaryWithDetailError) {
	projectName := types.StringNull()
	if workspaceGroup.ProjectID != nil {
		name, ok := projectNamesByID[workspaceGroup.ProjectID.String()]
		if !ok {
			return workspaceGroupDataSourceModel{}, &util.SummaryWithDetailError{
				Summary: "Failed to resolve project name",
				Detail:  "Unable to resolve project name for workspace group project ID '" + workspaceGroup.ProjectID.String() + "'.",
			}
		}
		projectName = types.StringValue(name)
	}

	return workspaceGroupDataSourceModel{
		ID:                       util.UUIDStringValue(workspaceGroup.WorkspaceGroupID),
		Name:                     types.StringValue(workspaceGroup.Name),
		ProjectName:              projectName,
		State:                    util.WorkspaceGroupStateStringValue(workspaceGroup.State),
		FirewallRanges:           util.FirewallRanges(workspaceGroup.FirewallRanges),
		AllowAllTraffic:          util.MaybeBoolValue(workspaceGroup.AllowAllTraffic),
		CreatedAt:                types.StringValue(workspaceGroup.CreatedAt),
		ExpiresAt:                util.MaybeStringValue(workspaceGroup.ExpiresAt),
		RegionID:                 util.UUIDStringValue(workspaceGroup.RegionID),
		CloudProvider:            types.StringValue(string(workspaceGroup.Provider)),
		RegionName:               types.StringValue(workspaceGroup.RegionName),
		UpdateWindow:             toUpdateWindowDataSourceModel(workspaceGroup.UpdateWindow),
		DeploymentType:           util.StringValueOrNull(workspaceGroup.DeploymentType),
		OptInPreviewFeature:      util.MaybeBoolValue(workspaceGroup.OptInPreviewFeature),
		HighAvailabilityTwoZones: util.MaybeBoolValue(workspaceGroup.HighAvailabilityTwoZones),
		OutboundAllowList:        util.MaybeStringValue(workspaceGroup.OutboundAllowList),
	}, nil
}

func getProjectNamesByID(ctx context.Context, c management.ClientWithResponsesInterface) (map[string]string, *util.SummaryWithDetailError) {
	projectsResponse, err := c.GetV1ProjectsWithResponse(ctx)
	if serr := util.StatusOK(projectsResponse, err, util.ReturnNilOnNotFound); serr != nil {
		return nil, serr
	}

	result := map[string]string{}
	for _, project := range util.Deref(projectsResponse.JSON200) {
		result[project.ProjectID.String()] = project.Name
	}

	return result, nil
}

func hasWorkspaceGroupWithProjectID(workspaceGroups []management.WorkspaceGroup) bool {
	for _, workspaceGroup := range workspaceGroups {
		if workspaceGroup.ProjectID != nil {
			return true
		}
	}

	return false
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
