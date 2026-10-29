package regions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	dataSourceListName = "regions"
)

// regionsDataSourceList is the data source implementation.
type regionsDataSourceList struct {
	management.ClientWithResponsesInterface
}

// regionsListDataSourceModel maps the data source schema data.
type regionsListDataSourceModel struct {
	ID      types.String  `tfsdk:"id"`
	Regions []regionModel `tfsdk:"regions"`
}

// regionModel maps regions schema data (aligned with RegionV2 / regions_v2).
type regionModel struct {
	ID         types.String `tfsdk:"id"`
	Provider   types.String `tfsdk:"provider"`
	Region     types.String `tfsdk:"region"`
	RegionName types.String `tfsdk:"region_name"`
}

var _ datasource.DataSourceWithConfigure = &regionsDataSourceList{}

// NewDataSourceList is a helper function to simplify the provider implementation.
func NewDataSourceList() datasource.DataSource {
	return &regionsDataSourceList{}
}

// Metadata returns the data source type name.
func (d *regionsDataSourceList) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, dataSourceListName)
}

// Schema defines the schema for the data source.
func (d *regionsDataSourceList) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "This data source provides a list of regions that the user can access and that support workspaces. It includes the region code name and provider for each region.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed: true,
			},
			dataSourceListName: schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						config.IDAttribute: schema.StringAttribute{
							Computed:            true,
							DeprecationMessage:  config.RegionIDDeprecationMessage,
							MarkdownDescription: "Deprecated. The unique identifier for the region. " + config.RegionIDDeprecationMessage,
						},
						"provider": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the cloud provider hosting the region.",
						},
						"region": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the region.",
						},
						"region_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The region code name.",
						},
					},
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *regionsDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	regions, err := d.GetV2RegionsWithResponse(ctx, &management.GetV2RegionsParams{})
	if serr := util.StatusOK(regions, err, util.ReturnNilOnNotFound); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	regionIDs := d.legacyRegionIDs(ctx)

	result := regionsListDataSourceModel{
		ID: types.StringValue(config.TestIDValue),
		Regions: util.Map(util.Deref(regions.JSON200), func(region management.RegionV2) regionModel {
			return toRegionsDataSourceModel(region, regionIDs)
		}),
	}

	resp.Diagnostics.AddWarning("Deprecated attribute regions[*].id", config.RegionIDDeprecationMessage)

	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

type legacyRegionKey struct {
	provider management.CloudProvider
	region   string
}

// legacyRegionIDs maps v2 regions to the v1 region IDs that back the deprecated regions[*].id.
// A failure leaves the IDs null instead of failing the read.
func (d *regionsDataSourceList) legacyRegionIDs(ctx context.Context) map[legacyRegionKey]string {
	regions, err := d.getV1Regions(ctx)
	if err != nil {
		tflog.Warn(ctx, "failed to read v1 regions for the deprecated regions[*].id attribute", map[string]any{
			"error": err.Error(),
		})

		return nil
	}

	result := make(map[legacyRegionKey]string, len(regions))
	for _, region := range regions {
		result[legacyRegionKey{provider: region.Provider, region: region.Region}] = region.RegionID.String()
	}

	return result
}

// getV1Regions calls GET /v1/regions directly because the management SDK no longer generates it.
func (d *regionsDataSourceList) getV1Regions(ctx context.Context) ([]management.Region, error) {
	withResponses, ok := d.ClientWithResponsesInterface.(*management.ClientWithResponses)
	if !ok {
		return nil, fmt.Errorf("unsupported management client %T", d.ClientWithResponsesInterface)
	}

	client, ok := withResponses.ClientInterface.(*management.Client)
	if !ok {
		return nil, fmt.Errorf("unsupported management client %T", withResponses.ClientInterface)
	}

	endpoint, err := url.JoinPath(client.Server, "v1", "regions")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	for _, edit := range client.RequestEditors {
		if err := edit(ctx, req); err != nil {
			return nil, err
		}
	}

	resp, err := client.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	var regions []management.Region
	if err := json.NewDecoder(resp.Body).Decode(&regions); err != nil {
		return nil, err
	}

	return regions, nil
}

// Configure adds the provider configured client to the data source.
func (d *regionsDataSourceList) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func toRegionsDataSourceModel(region management.RegionV2, regionIDs map[legacyRegionKey]string) regionModel {
	id := types.StringNull()
	if regionID, ok := regionIDs[legacyRegionKey{provider: region.Provider, region: region.Region}]; ok {
		id = types.StringValue(regionID)
	}

	return regionModel{
		ID:         id,
		Provider:   types.StringValue(string(region.Provider)),
		Region:     types.StringValue(region.Region),
		RegionName: types.StringValue(region.RegionName),
	}
}
