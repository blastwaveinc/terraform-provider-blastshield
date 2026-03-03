package provider

import (
	"context"
	"os"

	"github.com/blastwaveinc/terraform-provider-blastshield/internal/provider/versions"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &BlastshieldProvider{}

type BlastshieldProvider struct {
	version string
	vp      versions.VersionedProvider
}

type BlastshieldProviderModel struct {
	Host  types.String `tfsdk:"host"`
	Token types.String `tfsdk:"token"`
}

func (p *BlastshieldProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "blastshield"
	resp.Version = p.version
}

func (p *BlastshieldProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for managing Blastshield resources.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Description: "The Blastshield API host URL. Can also be set via the BLASTSHIELD_HOST environment variable.",
				Optional:    true,
			},
			"token": schema.StringAttribute{
				Description: "The Blastshield API token. Can also be set via the BLASTSHIELD_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *BlastshieldProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config BlastshieldProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check environment variables
	host := os.Getenv("BLASTSHIELD_HOST")
	token := os.Getenv("BLASTSHIELD_TOKEN")

	// Override with config values if set
	if !config.Host.IsNull() {
		host = config.Host.ValueString()
	}
	if !config.Token.IsNull() {
		token = config.Token.ValueString()
	}

	// Validate required values
	if host == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Missing Blastshield API Host",
			"The provider cannot create the Blastshield API client as there is a missing or empty value for the Blastshield API host. "+
				"Set the host value in the configuration or use the BLASTSHIELD_HOST environment variable.",
		)
	}

	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Missing Blastshield API Token",
			"The provider cannot create the Blastshield API client as there is a missing or empty value for the Blastshield API token. "+
				"Set the token value in the configuration or use the BLASTSHIELD_TOKEN environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Create API client
	client := NewClient(host, token)

	// Make the client available to resources and data sources
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *BlastshieldProvider) Resources(ctx context.Context) []func() resource.Resource {
	if p.vp != nil {
		return p.vp.Resources()
	}
	return nil
}

func (p *BlastshieldProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	if p.vp != nil {
		return p.vp.DataSources()
	}
	return nil
}

func New(version string, vp versions.VersionedProvider) func() provider.Provider {
	return func() provider.Provider {
		return &BlastshieldProvider{
			version: version,
			vp:      vp,
		}
	}
}
