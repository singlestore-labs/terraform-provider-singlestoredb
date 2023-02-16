package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// singlestoreProvider is the provider implementation.
type singlestoreProvider struct {
}

func New() func() provider.Provider {
	return func() provider.Provider {
		return &singlestoreProvider{}
	}
}

// Metadata returns the provider type name.
func (p *singlestoreProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "singlestore"
	resp.Version = "0.0.0"
}

// Schema defines the provider-level schema for configuration data.
func (p *singlestoreProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{}
}

// Configure prepares a SingleStore API client for data sources and resources.
func (p *singlestoreProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
}

// DataSources defines the data sources implemented in the provider.
func (p *singlestoreProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

// Resources defines the resources implemented in the provider.
func (p *singlestoreProvider) Resources(_ context.Context) []func() resource.Resource {
	return nil
}
