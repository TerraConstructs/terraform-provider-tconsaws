// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	fwtypes "github.com/terraconstructs/terraform-provider-tconsaws/internal/framework/types"
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
	Region                    types.String `tfsdk:"region"`
	AccessKey                 types.String `tfsdk:"access_key"`
	SecretKey                 types.String `tfsdk:"secret_key"`
	Token                     types.String `tfsdk:"token"`
	Profile                   types.String `tfsdk:"profile"`
	SharedCredentialsFiles    types.List   `tfsdk:"shared_credentials_files"`
	SharedConfigFiles         types.List   `tfsdk:"shared_config_files"`
	MaxRetries                types.Int64  `tfsdk:"max_retries"`
	RetryMode                 types.String `tfsdk:"retry_mode"`
	SkipMetadataApiCheck      types.Bool   `tfsdk:"skip_metadata_api_check"`
	Endpoints                 types.Set    `tfsdk:"endpoints"`
	AssumeRoleWithWebIdentity types.List   `tfsdk:"assume_role_with_web_identity"`
	AssumeRole                types.List   `tfsdk:"assume_role"`
}

type AssumeRoleWithWebIdentityModel struct {
	Duration             types.String `tfsdk:"duration"`
	Policy               types.String `tfsdk:"policy"`
	PolicyARNs           types.Set    `tfsdk:"policy_arns"`
	RoleARN              types.String `tfsdk:"role_arn"`
	SessionName          types.String `tfsdk:"session_name"`
	WebIdentityToken     types.String `tfsdk:"web_identity_token"`
	WebIdentityTokenFile types.String `tfsdk:"web_identity_token_file"`
}

type staticTokenRetriever string

func (s staticTokenRetriever) GetIdentityToken() ([]byte, error) {
	return []byte(s), nil
}

// AssumeRoleModel mirrors the `assume_role` block attributes.
type AssumeRoleModel struct {
	Duration          types.String `tfsdk:"duration"`
	Policy            types.String `tfsdk:"policy"`
	PolicyARNs        types.Set    `tfsdk:"policy_arns"`
	RoleARN           types.String `tfsdk:"role_arn"`
	SessionName       types.String `tfsdk:"session_name"`
	ExternalID        types.String `tfsdk:"external_id"`
	SourceIdentity    types.String `tfsdk:"source_identity"`
	Tags              types.Map    `tfsdk:"tags"`
	TransitiveTagKeys types.List   `tfsdk:"transitive_tag_keys"`
}

// ProviderData holds the configured clients for the provider.
type ProviderData struct {
	SQSClient *sqs.Client
}

func (p *TconsAwsProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "tconsaws"
	resp.Version = p.version
}

func (p *TconsAwsProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	// https://github.com/hashicorp/terraform-provider-aws/blob/v6.0.0/internal/provider/framework/provider.go#L101
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
		Blocks: map[string]schema.Block{
			"assume_role": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"duration": schema.StringAttribute{
							CustomType:  fwtypes.DurationType,
							Optional:    true,
							Description: "The duration, between 15 minutes and 12 hours, of the role session. Valid time units are ns, us (or µs), ms, s, h, or m.",
						},
						"external_id": schema.StringAttribute{
							Optional:    true,
							Description: "A unique identifier that might be required when you assume a role in another account.",
						},
						"policy": schema.StringAttribute{
							Optional:    true,
							Description: "IAM Policy JSON describing further restricting permissions for the IAM Role being assumed.",
						},
						"policy_arns": schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Amazon Resource Names (ARNs) of IAM Policies describing further restricting permissions for the IAM Role being assumed.",
						},
						"role_arn": schema.StringAttribute{
							Optional:    true, // For historical reasons, we allow an empty `assume_role` block
							Description: "Amazon Resource Name (ARN) of an IAM Role to assume prior to making API calls.",
						},
						"session_name": schema.StringAttribute{
							Optional:    true,
							Description: "An identifier for the assumed role session.",
						},
						"source_identity": schema.StringAttribute{
							Optional:    true,
							Description: "Source identity specified by the principal assuming the role.",
						},
						"tags": schema.MapAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Assume role session tags.",
						},
						"transitive_tag_keys": schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Assume role session tag keys to pass to any subsequent sessions.",
						},
					},
				},
			},
			"assume_role_with_web_identity": schema.ListNestedBlock{
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"duration": schema.StringAttribute{
							CustomType:  fwtypes.DurationType,
							Optional:    true,
							Description: "The duration, between 15 minutes and 12 hours, of the role session. Valid time units are ns, us (or µs), ms, s, h, or m.",
						},
						"policy": schema.StringAttribute{
							Optional:    true,
							Description: "IAM Policy JSON describing further restricting permissions for the IAM Role being assumed.",
						},
						"policy_arns": schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Amazon Resource Names (ARNs) of IAM Policies describing further restricting permissions for the IAM Role being assumed.",
						},
						"role_arn": schema.StringAttribute{
							Optional:    true, // For historical reasons, we allow an empty `assume_role_with_web_identity` block
							Description: "Amazon Resource Name (ARN) of an IAM Role to assume prior to making API calls.",
						},
						"session_name": schema.StringAttribute{
							Optional:    true,
							Description: "An identifier for the assumed role session.",
						},
						"web_identity_token": schema.StringAttribute{
							Optional:    true,
							Description: "Value of a web identity token from an OpenID Connect (OIDC) or OAuth provider. One of `web_identity_token` or `web_identity_token_file` is required.",
						},
						"web_identity_token_file": schema.StringAttribute{
							Optional:    true,
							Description: "File containing a web identity token from an OpenID Connect (OIDC) or OAuth provider. One of `web_identity_token` or `web_identity_token_file` is required.",
						},
					},
				},
			},
			"endpoints": endpointsBlock(),
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
		region := data.Region.ValueString()
		configOptions = append(configOptions, config.WithRegion(region))
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

	// handle `endpoints` set block
	endpoints := make(map[string]string)
	if !data.Endpoints.IsNull() && !data.Endpoints.IsUnknown() {
		var sets []EndpointsModel
		resp.Diagnostics.Append(
			data.Endpoints.ElementsAs(ctx, &sets, false)...,
		)
		if len(sets) > 0 {
			// first (and only) element
			expandEndpointsModel(endpoints, sets[0], resp)
		}
	}

	// Handle `assume_role_with_web_identity` nested block
	if !data.AssumeRoleWithWebIdentity.IsNull() && !data.AssumeRoleWithWebIdentity.IsUnknown() {
		var blocks []AssumeRoleWithWebIdentityModel
		resp.Diagnostics.Append(
			data.AssumeRoleWithWebIdentity.ElementsAs(ctx, &blocks, false)...,
		)
		if len(blocks) > 0 {
			// only one due to SizeAtMost(1)
			m := blocks[0]
			dur, err := time.ParseDuration(m.Duration.ValueString())
			if err != nil {
				resp.Diagnostics.AddError("Invalid duration", err.Error())
				return
			}
			// Create STS client with initial AWS configuration
			cfgBase, err := config.LoadDefaultConfig(ctx, configOptions...)
			if err != nil {
				resp.Diagnostics.AddError(
					"Unable to configure AWS SDK",
					"Error loading AWS configuration: "+err.Error(),
				)
				return
			}
			stsClient := sts.NewFromConfig(cfgBase)

			// Build token retriever
			var retriever stscreds.IdentityTokenRetriever
			// one of these is required
			if v := m.WebIdentityTokenFile.ValueString(); v != "" {
				retriever = stscreds.IdentityTokenFile(v)
			} else {
				retriever = staticTokenRetriever(m.WebIdentityToken.ValueString())
			}
			// Build the WithWebIdentityRole provider
			webProv := stscreds.NewWebIdentityRoleProvider(
				stsClient,
				m.RoleARN.ValueString(),
				retriever,
				func(o *stscreds.WebIdentityRoleOptions) {
					o.RoleARN = m.RoleARN.ValueString()
					o.RoleSessionName = m.SessionName.ValueString()
					policy := m.Policy.ValueString()
					if policy != "" {
						o.Policy = &policy
					}
					o.Duration = dur
					if !m.PolicyARNs.IsNull() && !m.PolicyARNs.IsUnknown() {
						var arns []string
						resp.Diagnostics.Append(
							m.PolicyARNs.ElementsAs(ctx, &arns, false)...,
						)
						if len(arns) > 0 {
							o.PolicyARNs = make([]ststypes.PolicyDescriptorType, len(arns))
							for i, arn := range arns {
								o.PolicyARNs[i] = ststypes.PolicyDescriptorType{
									Arn: aws.String(arn),
								}
							}
						}
					}
				},
			)
			// Inject into LoadDefaultConfig
			configOptions = append(configOptions,
				config.WithCredentialsProvider(aws.NewCredentialsCache(webProv)),
			)
		}
	}

	// Handle `assume_role` nested block
	if !data.AssumeRole.IsNull() && !data.AssumeRole.IsUnknown() {
		var blocks []AssumeRoleModel
		resp.Diagnostics.Append(
			data.AssumeRole.ElementsAs(ctx, &blocks, false)...,
		)
		if len(blocks) > 0 {
			// only one due to SizeAtMost(1)
			m := blocks[0]
			dur, err := time.ParseDuration(m.Duration.ValueString())
			if err != nil {
				resp.Diagnostics.AddError("Invalid duration", err.Error())
				return
			}

			// Create STS client with initial AWS configuration
			cfgBase, err := config.LoadDefaultConfig(ctx, configOptions...)
			if err != nil {
				resp.Diagnostics.AddError(
					"Unable to configure AWS SDK",
					"Error loading AWS configuration: "+err.Error(),
				)
				return
			}
			stsClient := sts.NewFromConfig(cfgBase)

			// Construct AssumeRole provider
			assumeProvider := stscreds.NewAssumeRoleProvider(
				stsClient,
				m.RoleARN.ValueString(),
				func(o *stscreds.AssumeRoleOptions) {
					o.Duration = dur
					o.RoleSessionName = m.SessionName.ValueString()
					o.ExternalID = aws.String(m.ExternalID.ValueString())
					o.SourceIdentity = aws.String(m.SourceIdentity.ValueString())
					if policy := m.Policy.ValueString(); policy != "" {
						o.Policy = &policy
					}
					if !m.PolicyARNs.IsNull() && !m.PolicyARNs.IsUnknown() {
						var arns []string
						resp.Diagnostics.Append(
							m.PolicyARNs.ElementsAs(ctx, &arns, false)...,
						)
						for _, arn := range arns {
							o.PolicyARNs = append(o.PolicyARNs,
								ststypes.PolicyDescriptorType{Arn: aws.String(arn)},
							)
						}
					}
					// Convert map[string]string from Tags
					if !m.Tags.IsNull() && !m.Tags.IsUnknown() {
						var tags map[string]string
						resp.Diagnostics.Append(
							m.Tags.ElementsAs(ctx, &tags, false)...,
						)
						var tagList []ststypes.Tag
						for k, v := range tags {
							tagList = append(tagList, ststypes.Tag{
								Key:   aws.String(k),
								Value: aws.String(v),
							})
						}
						o.Tags = tagList
					}
					// Convert []string from TransitiveTagKeys
					if !m.TransitiveTagKeys.IsNull() && !m.TransitiveTagKeys.IsUnknown() {
						var keys []string
						resp.Diagnostics.Append(
							m.TransitiveTagKeys.ElementsAs(ctx, &keys, false)...,
						)
						o.TransitiveTagKeys = keys
					}
				},
			)

			// Inject as credentials provider
			configOptions = append(configOptions,
				config.WithCredentialsProvider(aws.NewCredentialsCache(assumeProvider)),
			)
		}
	}

	// Final LoadDefaultConfig
	cfg, err := config.LoadDefaultConfig(ctx, configOptions...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to configure AWS SDK",
			"Error loading AWS configuration: "+err.Error(),
		)
		return
	}
	var sqsClient *sqs.Client
	if sqsEndpoint, ok := endpoints[sqs.ServiceID]; ok {
		sqsClient = sqs.NewFromConfig(cfg, func(o *sqs.Options) {
			o.BaseEndpoint = aws.String(sqsEndpoint) // << set your custom URL
		})
	} else {
		sqsClient = sqs.NewFromConfig(cfg)
	}

	// Create provider data
	providerData := &ProviderData{
		SQSClient: sqsClient,
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
