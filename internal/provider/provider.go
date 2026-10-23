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
	Host        types.String `tfsdk:"host"`
	Token       types.String `tfsdk:"token"`
	DefaultTags types.Map    `tfsdk:"default_tags"`
}

// ProviderData is passed to resources and data sources: the API client plus
// provider-level settings.
type ProviderData struct {
	*Client
	defaultTags      map[string]string
	defaultTagsKnown bool
}

// DefaultTags returns the provider default_tags. known is false while the
// provider configuration still contains unknown values.
func (d *ProviderData) DefaultTags() (map[string]string, bool) {
	return d.defaultTags, d.defaultTagsKnown
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
			"default_tags": schema.MapAttribute{
				Description: "Tags applied to all resources that support tags. Resource-level tags take precedence on key conflicts. " +
					"The merged result is exposed in each resource's tags_all attribute.",
				ElementType: types.StringType,
				Optional:    true,
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

	data := &ProviderData{
		Client:           NewClient(host, token),
		defaultTagsKnown: true,
	}
	if config.DefaultTags.IsUnknown() {
		data.defaultTagsKnown = false
	} else {
		for _, v := range config.DefaultTags.Elements() {
			if v.IsUnknown() {
				data.defaultTagsKnown = false
			}
		}
	}
	if data.defaultTagsKnown && !config.DefaultTags.IsNull() {
		data.defaultTags = make(map[string]string)
		for k, v := range config.DefaultTags.Elements() {
			// A null value (e.g. from an unset variable) leaves the tag out
			if s, ok := v.(types.String); ok && !s.IsNull() {
				data.defaultTags[k] = s.ValueString()
			}
		}
	}

	// Make the client available to resources and data sources
	resp.DataSourceData = data
	resp.ResourceData = data
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
