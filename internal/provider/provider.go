// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure TconsAwsProvider satisfies various provider interfaces.
var _ provider.Provider = &TconsAwsProvider{}
var _ provider.ProviderWithFunctions = &TconsAwsProvider{}
var _ provider.ProviderWithEphemeralResources = &TconsAwsProvider{}

// TconsAwsProvider defines the provider implementation.
type TconsAwsProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// TconsAwsProviderModel describes the provider data model.
type TconsAwsProviderModel struct {
	Region                 types.String `tfsdk:"region"`
	AccessKey              types.String `tfsdk:"access_key"`
	SecretKey              types.String `tfsdk:"secret_key"`
	Token                  types.String `tfsdk:"token"`
	Profile                types.String `tfsdk:"profile"`
	SharedCredentialsFiles types.List   `tfsdk:"shared_credentials_files"`
	SharedConfigFiles      types.List   `tfsdk:"shared_config_files"`
	MaxRetries             types.Int64  `tfsdk:"max_retries"`
	RetryMode              types.String `tfsdk:"retry_mode"`
	SkipMetadataApiCheck   types.Bool   `tfsdk:"skip_metadata_api_check"`
}

// ProviderData holds the configured clients for the provider.
type ProviderData struct {
	SQSClient  *sqs.Client
	IMDSClient *imds.Client
}

func (p *TconsAwsProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "tconsaws"
	resp.Version = p.version
}

func (p *TconsAwsProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The terraform-provider-tconsaws provider enables CloudFormation cfn-signal equivalent functionality for Terraform using AWS SQS.",
		Attributes: map[string]schema.Attribute{
			"region": schema.StringAttribute{
				MarkdownDescription: "Default AWS region for resources",
				Optional:            true,
			},
			"access_key": schema.StringAttribute{
				MarkdownDescription: "AWS access key ID",
				Optional:            true,
				Sensitive:           true,
			},
			"secret_key": schema.StringAttribute{
				MarkdownDescription: "AWS secret access key",
				Optional:            true,
				Sensitive:           true,
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "AWS session token",
				Optional:            true,
				Sensitive:           true,
			},
			"profile": schema.StringAttribute{
				MarkdownDescription: "AWS shared configuration profile name",
				Optional:            true,
			},
			"shared_credentials_files": schema.ListAttribute{
				MarkdownDescription: "List of paths to AWS shared credentials files",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"shared_config_files": schema.ListAttribute{
				MarkdownDescription: "List of paths to AWS shared configuration files",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"max_retries": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of retries for AWS API calls",
				Optional:            true,
			},
			"retry_mode": schema.StringAttribute{
				MarkdownDescription: "AWS retry mode (legacy, standard, adaptive)",
				Optional:            true,
			},
			"skip_metadata_api_check": schema.BoolAttribute{
				MarkdownDescription: "Skip EC2 instance metadata service reachability check",
				Optional:            true,
			},
		},
	}
}

func (p *TconsAwsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data TconsAwsProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build AWS config using SDK v2 default config loader
	configOptions := []func(*config.LoadOptions) error{}

	// Handle region
	if !data.Region.IsNull() && !data.Region.IsUnknown() {
		configOptions = append(configOptions, config.WithRegion(data.Region.ValueString()))
	}

	// Handle static credentials
	if !data.AccessKey.IsNull() && !data.SecretKey.IsNull() {
		accessKey := data.AccessKey.ValueString()
		secretKey := data.SecretKey.ValueString()
		token := ""
		if !data.Token.IsNull() {
			token = data.Token.ValueString()
		}
		configOptions = append(configOptions, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, token),
		))
	}

	// Handle profile
	if !data.Profile.IsNull() && !data.Profile.IsUnknown() {
		configOptions = append(configOptions, config.WithSharedConfigProfile(data.Profile.ValueString()))
	}

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx, configOptions...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to configure AWS SDK",
			"Error loading AWS configuration: "+err.Error(),
		)
		return
	}

	// Create SQS client
	sqsClient := sqs.NewFromConfig(cfg)

	// Create IMDS client
	imdsClient := imds.NewFromConfig(cfg)

	// Create provider data
	providerData := &ProviderData{
		SQSClient:  sqsClient,
		IMDSClient: imdsClient,
	}

	tflog.Info(ctx, "Configured AWS SDK for tconsaws provider")

	resp.DataSourceData = providerData
	resp.ResourceData = providerData
}

func (p *TconsAwsProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSignalResource,
	}
}

func (p *TconsAwsProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{}
}

func (p *TconsAwsProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *TconsAwsProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &TconsAwsProvider{
			version: version,
		}
	}
}
