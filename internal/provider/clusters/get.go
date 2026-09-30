package clusters

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	DataSourceGetName = "cluster"
)

// clustersDataSourceGet is the data source implementation.
type clustersDataSourceGet struct {
	management.ClientWithResponsesInterface
}

// clusterDataSourceModel maps cluster schema data.
type clusterDataSourceModel struct {
	ID                  types.String                 `tfsdk:"id"`
	Name                types.String                 `tfsdk:"name"`
	GroupID             types.String                 `tfsdk:"group_id"`
	State               types.String                 `tfsdk:"state"`
	Size                types.String                 `tfsdk:"size"`
	ScaleFactor         types.Float32                `tfsdk:"scale_factor"`
	CacheConfig         types.Float32                `tfsdk:"cache_config"`
	Suspended           types.Bool                   `tfsdk:"suspended"`
	Kai                 types.Bool                   `tfsdk:"kai"`
	FirewallRanges      []types.String               `tfsdk:"firewall_ranges"`
	ExpiresAt           types.String                 `tfsdk:"expires_at"`
	CloudProvider       types.String                 `tfsdk:"cloud_provider"`
	RegionName          types.String                 `tfsdk:"region_name"`
	DeploymentType      types.String                 `tfsdk:"deployment_type"`
	OptInPreviewFeature types.Bool                   `tfsdk:"opt_in_preview_feature"`
	MultiAZ             types.Bool                   `tfsdk:"multi_az"`
	OutboundAllowList   types.String                 `tfsdk:"outbound_allow_list"`
	Endpoint            types.String                 `tfsdk:"endpoint"`
	CreatedAt           types.String                 `tfsdk:"created_at"`
	AutoScale           *autoScaleResourceModel      `tfsdk:"auto_scale"`
	AutoSuspend         *autoSuspendResourceModel    `tfsdk:"auto_suspend"`
	UpdateWindow        *updateWindowDataSourceModel `tfsdk:"update_window"`
}

type updateWindowDataSourceModel struct {
	Hour types.Int64 `tfsdk:"hour"`
	Day  types.Int64 `tfsdk:"day"`
}

type clusterDataSourceSchemaConfig struct {
	computeClusterID    bool
	requireClusterID    bool
	computeName         bool
	clusterIDValidators []validator.String
}

var _ datasource.DataSourceWithConfigure = &clustersDataSourceGet{}

// NewDataSourceGet is a helper function to simplify the provider implementation.
func NewDataSourceGet() datasource.DataSource {
	return &clustersDataSourceGet{}
}

// Metadata returns the data source type name.
func (d *clustersDataSourceGet) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceGetName)
}

// Schema defines the schema for the data source.
func (d *clustersDataSourceGet) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieve a specific cluster using its ID with this data source.",
		Attributes: newClusterDataSourceSchemaAttributes(clusterDataSourceSchemaConfig{
			requireClusterID:    true,
			computeName:         true,
			clusterIDValidators: []validator.String{util.NewUUIDValidator()},
		}),
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *clustersDataSourceGet) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data clusterDataSourceModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := uuid.MustParse(data.ID.ValueString())
	cluster, err := d.GetV2ClustersClusterIDWithResponse(ctx, id, &management.GetV2ClustersClusterIDParams{})
	if serr := util.StatusOK(cluster, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	result := toClusterDataSourceModel(*cluster.JSON200)
	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *clustersDataSourceGet) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func toClusterDataSourceModel(cluster management.Cluster) clusterDataSourceModel {
	size := types.StringNull()
	scaleFactor := types.Float32Null()
	cacheConfig := types.Float32Null()
	if cluster.SizeConfig != nil {
		size = util.MaybeStringValue(cluster.SizeConfig.Size)
		scaleFactor = types.Float32PointerValue(cluster.SizeConfig.ScaleFactor)
		cacheConfig = types.Float32PointerValue(cluster.SizeConfig.CacheConfig)
	}

	return clusterDataSourceModel{
		ID:                  util.MaybeUUIDStringValue(cluster.ClusterID),
		Name:                types.StringValue(cluster.Name),
		GroupID:             util.MaybeUUIDStringValue(cluster.GroupID),
		State:               util.ClusterStateStringValue(util.Deref(cluster.State)),
		Size:                size,
		ScaleFactor:         scaleFactor,
		CacheConfig:         cacheConfig,
		Suspended:           types.BoolValue(util.Deref(cluster.State) == management.ClusterStateSUSPENDED),
		Kai:                 util.MaybeBoolValue(cluster.Kai),
		FirewallRanges:      util.FirewallRanges(cluster.FirewallRanges),
		ExpiresAt:           util.MaybeStringValue(cluster.ExpiresAt),
		CloudProvider:       normalizeCloudProvider(cluster.Provider),
		RegionName:          util.MaybeStringValue(cluster.Region),
		DeploymentType:      util.StringValueOrNull(cluster.DeploymentType),
		OptInPreviewFeature: util.MaybeBoolValue(cluster.OptInPreviewFeature),
		MultiAZ:             util.MaybeBoolValue(cluster.MultiAZ),
		OutboundAllowList:   util.MaybeStringValue(cluster.OutboundAllowList),
		Endpoint:            util.MaybeStringValue(cluster.Endpoint),
		CreatedAt:           util.MaybeTimeValue(cluster.CreatedAt),
		AutoScale:           toAutoScaleResourceModel(cluster),
		AutoSuspend:         toAutoSuspendResourceModel(cluster),
		UpdateWindow:        toUpdateWindowDataSourceModel(cluster.UpdateWindow),
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

func newClusterDataSourceSchemaAttributes(conf clusterDataSourceSchemaConfig) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		config.IDAttribute: schema.StringAttribute{
			Computed:            conf.computeClusterID,
			Required:            conf.requireClusterID,
			MarkdownDescription: "The unique identifier of the cluster.",
			Validators:          conf.clusterIDValidators,
		},
		"name": schema.StringAttribute{
			Computed:            conf.computeName,
			MarkdownDescription: "The name of the cluster.",
		},
		"group_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The unique identifier of the workspace group that contains the cluster.",
		},
		"state": schema.StringAttribute{
			Computed: true,
			MarkdownDescription: fmt.Sprintf("The state of the cluster. Possible values are %s.", util.Join([]management.ClusterState{
				management.ClusterStateACTIVE,
				management.ClusterStateFAILED,
				management.ClusterStatePENDING,
				management.ClusterStateSUSPENDED,
				management.ClusterStateTERMINATED,
				management.ClusterStateUNKNOWN,
			}, ", ")),
		},
		"size": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The size of the cluster, specified in workspace size notation (S-00, S-0, S-1, S-2).",
		},
		"scale_factor": schema.Float32Attribute{
			Computed:            true,
			MarkdownDescription: "The scale factor of the cluster.",
		},
		"cache_config": schema.Float32Attribute{
			Computed:            true,
			MarkdownDescription: "The cache config multiplier of the cluster.",
		},
		"suspended": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether the cluster is suspended.",
		},
		"kai": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether SingleStore Kai is enabled for the cluster.",
		},
		"firewall_ranges": schema.ListAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "List of allowed CIDR ranges.",
		},
		"expires_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The expiration timestamp of the cluster.",
		},
		"cloud_provider": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The cloud provider of the cluster.",
		},
		"region_name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The region code of the cluster.",
		},
		"deployment_type": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The deployment type of the cluster.",
		},
		"opt_in_preview_feature": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether preview features are enabled.",
		},
		"multi_az": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether the cluster is deployed across multiple availability zones.",
		},
		"outbound_allow_list": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The outbound allow list identifier (AWS only).",
		},
		"endpoint": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The endpoint used to connect to the cluster.",
		},
		"created_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The timestamp when the cluster was created.",
		},
		"auto_scale": schema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Autoscale settings for the cluster.",
			Attributes: map[string]schema.Attribute{
				"max_scale_factor": schema.Float32Attribute{
					Computed:            true,
					MarkdownDescription: "The maximum scale factor allowed for the cluster.",
				},
				"sensitivity": schema.StringAttribute{
					Computed:            true,
					MarkdownDescription: "The sensitivity of the autoscale operation.",
				},
			},
		},
		"auto_suspend": schema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Auto suspend settings for the cluster.",
			Attributes: map[string]schema.Attribute{
				"suspend_after_seconds": schema.Float32Attribute{
					Computed:            true,
					MarkdownDescription: "When to suspend the cluster, according to the suspend type chosen.",
				},
				"suspend_type": schema.StringAttribute{
					Computed:            true,
					MarkdownDescription: "The auto suspend mode for the cluster.",
				},
			},
		},
		"update_window": schema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "The scheduled update window for the cluster.",
			Attributes: map[string]schema.Attribute{
				"hour": schema.Int64Attribute{
					Computed:            true,
					MarkdownDescription: "The hour of the day (UTC) when the update window starts.",
				},
				"day": schema.Int64Attribute{
					Computed:            true,
					MarkdownDescription: "The day of the week when the update window is scheduled.",
				},
			},
		},
	}
}
