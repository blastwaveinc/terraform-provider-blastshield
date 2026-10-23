package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/blastwaveinc/terraform-provider-blastshield/internal/provider/versions"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &BlastshieldProvider{}

type BlastshieldProvider struct {
	version string
	sel     versions.Selection
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

	resp.Diagnostics.Append(p.checkAPIVersion(host, token)...)
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

// checkAPIVersion checks that the API version the provider was started with is the
// one the configured server needs. The version is chosen at startup, before
// Terraform sends the provider configuration, so it can't be switched here.
func (p *BlastshieldProvider) checkAPIVersion(host, token string) diag.Diagnostics {
	var diags diag.Diagnostics
	if p.sel.Err != nil {
		diags.AddError("Unsupported BLASTSHIELD_API_VERSION",
			fmt.Sprintf("%s. Set BLASTSHIELD_API_VERSION to a supported API version, or unset it.", p.sel.Err))
		return diags
	}
	if p.sel.Source == versions.SourceDetected && p.sel.Host == host {
		// Detected from this server at startup
		return diags
	}
	serverVersion, err := versions.FetchAPIVersion(host, token)
	if err != nil {
		diags.AddWarning("Unable to Verify the Blastshield API Version",
			fmt.Sprintf("Could not fetch the API version from %s: %s. The provider uses API version %s.", host, err, p.sel.Version))
		return diags
	}
	return apiVersionMismatch(p.sel, host, serverVersion)
}

// apiVersionMismatch reports when the server needs a different API version than
// the one in use, or runs a version that isn't supported: a warning when it was
// pinned with BLASTSHIELD_API_VERSION, an error otherwise.
func apiVersionMismatch(sel versions.Selection, host, serverVersion string) diag.Diagnostics {
	var diags diag.Diagnostics
	_, want, err := versions.SelectVersion(serverVersion)
	if err != nil {
		if sel.Source == versions.SourcePinned {
			diags.AddWarning("Unsupported Blastshield API Version",
				fmt.Sprintf("The server at %s runs API version %s: %s. BLASTSHIELD_API_VERSION selects API version %s.",
					host, serverVersion, err, sel.Version))
			return diags
		}
		diags.AddError("Unsupported Blastshield API Version",
			fmt.Sprintf("The server at %s runs API version %s: %s. To use it anyway, set BLASTSHIELD_API_VERSION to the API version to use.",
				host, serverVersion, err))
		return diags
	}
	if want == sel.Version {
		return diags
	}

	if sel.Source == versions.SourcePinned {
		diags.AddWarning("Pinned Blastshield API Version Differs From the Server",
			fmt.Sprintf("BLASTSHIELD_API_VERSION selects API version %s, but the server at %s runs API version %s, which needs API version %s. "+
				"Resources and attributes that the server doesn't support will fail.",
				sel.Version, host, serverVersion, want))
		return diags
	}

	reason := "No API version was detected when the provider started"
	if sel.Source == versions.SourceDetected {
		reason = fmt.Sprintf("The API version was detected from BLASTSHIELD_HOST (%s)", sel.Host)
	}
	diags.AddError("Blastshield API Version Mismatch",
		fmt.Sprintf("The provider uses API version %s, but the server at %s runs API version %s, which needs API version %s. "+
			"%s. The API version is chosen when the provider starts, before Terraform sends the provider configuration, "+
			"so host and token in the provider block can't choose it. Set BLASTSHIELD_HOST and BLASTSHIELD_TOKEN to this "+
			"server, or set BLASTSHIELD_API_VERSION=%s.",
			sel.Version, host, serverVersion, want, reason, want))
	return diags
}

func (p *BlastshieldProvider) Resources(ctx context.Context) []func() resource.Resource {
	if p.sel.Provider != nil {
		return p.sel.Provider.Resources()
	}
	return nil
}

func (p *BlastshieldProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	if p.sel.Provider != nil {
		return p.sel.Provider.DataSources()
	}
	return nil
}

func New(version string, sel versions.Selection) func() provider.Provider {
	return func() provider.Provider {
		return &BlastshieldProvider{
			version: version,
			sel:     sel,
		}
	}
}
