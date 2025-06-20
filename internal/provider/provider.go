package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/invitations"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/privateconnections"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/regions"
	regions_v2 "github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/regionsv2"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/roles"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/teams"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/users"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/workspacegroups"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/workspaces"
)

// singlestoreProvider is the provider implementation.
type singlestoreProvider struct {
	version string
}

// singlestoreProviderModel maps provider schema data to a Go type.
type singlestoreProviderModel struct {
	APIKey        types.String `tfsdk:"api_key"`
	APIKeyPath    types.String `tfsdk:"api_key_path"`
	APIServiceURL types.String `tfsdk:"api_service_url"`
}

var (
	_ provider.Provider                   = &singlestoreProvider{}
	_ provider.ProviderWithValidateConfig = &singlestoreProvider{}
)

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &singlestoreProvider{
			version: version,
		}
	}
}

// Metadata returns the provider type name.
func (p *singlestoreProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = config.ProviderName
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *singlestoreProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Terraform provider plugin for managing SingleStoreDB workspace groups and workspaces.",
		Attributes: map[string]schema.Attribute{
			config.APIKeyAttribute: schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("The SingleStore Management API key used for authentication. If not provided, the provider will attempt to read the key from the file specified in the '%s' attribute or from the environment variable '%s'. Generate your API key in the SingleStore Portal at %s.", config.APIKeyPathAttribute, config.EnvAPIKey, config.PortalAPIKeysPageRedirect),
				Optional:            true,
				Sensitive:           true,
			},
			config.APIKeyPathAttribute: schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("The absolute path to a file containing the SingleStore Management API key for authentication. If not provided, the provider will use the value in the '%s' attribute or the '%s' environment variable. Generate your API key in the SingleStore Portal at %s.", config.APIKeyAttribute, config.EnvAPIKey, config.PortalAPIKeysPageRedirect),
				Optional:            true,
				Sensitive:           true,
			},
			config.APIServiceURLAttribute: schema.StringAttribute{
				MarkdownDescription: "The URL of the SingleStore Management API service. This URL is used by the provider to interact with the API.",
				Optional:            true,
				DeprecationMessage:  "The use of the API service URL is now optional and is intended for testing purposes only.",
			},
		},
	}
}

// Configure prepares a SingleStore API client for data sources and resources.
func (p *singlestoreProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	// Retrieve provider data from configuration.
	var conf singlestoreProviderModel
	diags := req.Config.Get(ctx, &conf)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv(config.EnvAPIKey)

	if !conf.APIKeyPath.IsNull() {
		var err error
		apiKey, err = util.ReadNotEmptyFileTrimmed(conf.APIKeyPath.ValueString())
		if err != nil {
			resp.Diagnostics.AddAttributeError(
				path.Root(config.APIKeyPathAttribute),
				err.Error(),
				config.InvalidAPIKeyErrorDetail,
			)

			return
		}
	}

	if !conf.APIKey.IsNull() {
		apiKey = conf.APIKey.ValueString()
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.APIKeyAttribute),
			"Missing SingleStore API key",
			"The provider cannot create the SingleStore API client as there is a missing or empty value for the SingleStore API key. "+
				config.InvalidAPIKeyErrorDetail,
		)

		return
	}

	apiServiceURL := config.APIServiceURL

	if !conf.APIServiceURL.IsNull() {
		apiServiceURL = conf.APIServiceURL.ValueString()
	}

	client, err := management.NewClientWithResponses(apiServiceURL,
		management.WithHTTPClient(util.NewHTTPClient()),
		management.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
			req.Header.Set("User-Agent", util.TerraformProviderUserAgent(p.version))

			return nil
		}),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create SingleStore API client",
			"An unexpected error occurred when creating the SingleStore API client. "+
				config.InvalidAPIKeyErrorDetail+
				config.CreateProviderIssueIfNotClearErrorDetail+
				"\n\nSingleStore client error: "+err.Error(),
		)

		return
	}

	// Make the SingleStore client available during DataSource and Resource
	// type Configure methods.
	resp.DataSourceData = client
	resp.ResourceData = client
}

// DataSources defines the data sources implemented in the provider.
func (p *singlestoreProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		regions.NewDataSourceList,
		regions_v2.NewDataSourceList,
		workspacegroups.NewDataSourceList,
		workspacegroups.NewDataSourceGet,
		workspaces.NewDataSourceList,
		workspaces.NewDataSourceGet,
		privateconnections.NewDataSourceList,
		privateconnections.NewDataSourceGet,
		users.NewDataSourceGet,
		users.NewDataSourceList,
		invitations.NewDataSourceList,
		invitations.NewDataSourceGet,
		teams.NewDataSourceList,
		teams.NewDataSourceGet,
		roles.NewUserRolesDataSourceList,
		roles.NewRolesDataSourceList,
	}
}

// Resources defines the resources implemented in the provider.
func (p *singlestoreProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		workspacegroups.NewResource,
		workspaces.NewResource,
		privateconnections.NewResource,
		users.NewResource,
		teams.NewResource,
		roles.NewUserRoleGrantResource,
		roles.NewUserRolesGrantResource,
	}
}

// ValidateConfig asserts that incompatible fields are not specified.
func (p *singlestoreProvider) ValidateConfig(ctx context.Context, req provider.ValidateConfigRequest, resp *provider.ValidateConfigResponse) {
	// Retrieve provider data from configuration.
	var conf singlestoreProviderModel
	diags := req.Config.Get(ctx, &conf)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if conf.APIServiceURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.APIServiceURLAttribute),
			"Unknown Management API url",
			"The provider cannot create the Management API client as there is an unknown configuration value for the API server URL. "+
				fmt.Sprintf("Not indicate the %s attribute of the provider or set it to %s, which is the default Management API URL.", config.APIServiceURLAttribute, config.APIServiceURL),
		)

		return
	}

	if !conf.APIKey.IsNull() && !conf.APIKeyPath.IsNull() {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Cannot specify both '%s' and '%s'", config.APIKeyAttribute, config.APIKeyPathAttribute),
			config.InvalidAPIKeyErrorDetail,
		)

		return
	}
}
