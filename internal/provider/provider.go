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
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/regions"
)

// singlestoreProvider is the provider implementation.
type singlestoreProvider struct {
}

// singlestoreProviderModel maps provider schema data to a Go type.
type singlestoreProviderModel struct {
	APIKey        types.String `tfsdk:"api_key"`
	APIServiceURL types.String `tfsdk:"api_service_url"`
}

var _ provider.Provider = &singlestoreProvider{}

func New() func() provider.Provider {
	return func() provider.Provider {
		return &singlestoreProvider{}
	}
}

// Metadata returns the provider type name.
func (p *singlestoreProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = config.ProviderName
	resp.Version = config.Version
}

// Schema defines the provider-level schema for configuration data.
func (p *singlestoreProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			config.APIKeyAttribute: schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
			},
			config.APIServiceURLAttribute: schema.StringAttribute{
				Optional: true,
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

	if conf.APIKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.APIKeyAttribute),
			"Unknown API key",
			"The provider cannot create the Management API client as there is an unknown configuration value for the API key. "+
				fmt.Sprintf("Either target apply the source of the value first, set the value statically in the configuration, or use the %s environment variable.", config.EnvAPIKey),
		)

		return
	}

	apiKey := os.Getenv(config.EnvAPIKey)

	if !conf.APIKey.IsNull() {
		apiKey = conf.APIKey.ValueString()
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.APIKeyAttribute),
			"Missing SingleStore API key",
			"The provider cannot create the SingleStore API client as there is a missing or empty value for the SingleStore API key. "+
				fmt.Sprintf("Set the %s value in the configuration or use the %s environment variable. ", config.APIKeyAttribute, config.EnvAPIKey)+
				"If either is already set, ensure the value is not empty.",
		)

		return
	}

	if conf.APIServiceURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.APIServiceURLAttribute),
			"Unknown Management API url",
			"The provider cannot create the Management API client as there is an unknown configuration value for the API server url.",
		)

		return
	}

	apiServiceURL := config.APIServiceURL

	if !conf.APIServiceURL.IsNull() {
		apiServiceURL = conf.APIServiceURL.ValueString()
	}

	client, err := management.NewClientWithResponses(apiServiceURL,
		management.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
			return nil
		}),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create SingleStore API client",
			"An unexpected error occurred when creating the SingleStore API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"SingleStore client error: "+err.Error(),
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
		regions.NewDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *singlestoreProvider) Resources(_ context.Context) []func() resource.Resource {
	return nil
}
