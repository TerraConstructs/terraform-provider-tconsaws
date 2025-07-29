# AWS Provider Parity Gaps Analysis

## Overview

This document details the missing configuration attributes and implementation gaps between the current `terraform-provider-tconsaws` and full AWS provider parity based on `hashicorp/aws-sdk-go-base`.

## Current Status

### Implemented Features ✅
- Basic authentication (access_key, secret_key, token)
- Profile-based authentication
- Region configuration
- Basic retry configuration (max_retries)
- Assume role support
- Assume role with web identity support
- Custom endpoints (basic SQS endpoint override)

### Missing Schema Attributes ❌

#### 1. Account Management
```go
// Add to TconsAwsProviderModel
AllowedAccountIds    types.List `tfsdk:"allowed_account_ids"`
ForbiddenAccountIds  types.List `tfsdk:"forbidden_account_ids"`
SkipCredentialsValidation     types.Bool `tfsdk:"skip_credentials_validation"`
SkipRequestingAccountId       types.Bool `tfsdk:"skip_requesting_account_id"`
```

**Schema additions needed:**
```go
"allowed_account_ids": schema.ListAttribute{
    MarkdownDescription: "List of allowed AWS account IDs to restrict provider usage",
    Optional:            true,
    ElementType:         types.StringType,
},
"forbidden_account_ids": schema.ListAttribute{
    MarkdownDescription: "List of forbidden AWS account IDs to prevent provider usage", 
    Optional:            true,
    ElementType:         types.StringType,
},
"skip_credentials_validation": schema.BoolAttribute{
    MarkdownDescription: "Skip AWS credentials validation during provider initialization",
    Optional:            true,
},
"skip_requesting_account_id": schema.BoolAttribute{
    MarkdownDescription: "Skip requesting AWS account ID during provider initialization", 
    Optional:            true,
},
```

#### 2. HTTP/Proxy Configuration
```go
// Add to TconsAwsProviderModel
HttpProxy     types.String `tfsdk:"http_proxy"`
HttpsProxy    types.String `tfsdk:"https_proxy"`
NoProxy       types.String `tfsdk:"no_proxy"`
Insecure      types.Bool   `tfsdk:"insecure"`
CustomCABundle types.String `tfsdk:"custom_ca_bundle"`
```

**Schema additions needed:**
```go
"http_proxy": schema.StringAttribute{
    MarkdownDescription: "URL of HTTP proxy for AWS API requests",
    Optional:            true,
},
"https_proxy": schema.StringAttribute{
    MarkdownDescription: "URL of HTTPS proxy for AWS API requests",
    Optional:            true,
},
"no_proxy": schema.StringAttribute{
    MarkdownDescription: "Comma-separated list of hosts to bypass proxy for",
    Optional:            true,
},
"insecure": schema.BoolAttribute{
    MarkdownDescription: "Skip TLS certificate verification (NOT recommended for production)",
    Optional:            true,
},
"custom_ca_bundle": schema.StringAttribute{
    MarkdownDescription: "Path to custom CA bundle file for TLS verification",
    Optional:            true,
},
```

#### 3. EC2 Metadata Service (IMDS) Configuration
```go
// Add to TconsAwsProviderModel
EC2MetadataServiceEnableState  types.String `tfsdk:"ec2_metadata_service_enable_state"`
EC2MetadataServiceEndpoint     types.String `tfsdk:"ec2_metadata_service_endpoint"`
EC2MetadataServiceEndpointMode types.String `tfsdk:"ec2_metadata_service_endpoint_mode"`
```

**Schema additions needed:**
```go
"ec2_metadata_service_enable_state": schema.StringAttribute{
    MarkdownDescription: "State of EC2 metadata service (enabled, disabled)",
    Optional:            true,
},
"ec2_metadata_service_endpoint": schema.StringAttribute{
    MarkdownDescription: "Custom endpoint for EC2 metadata service",
    Optional:            true,
},
"ec2_metadata_service_endpoint_mode": schema.StringAttribute{
    MarkdownDescription: "Endpoint mode for EC2 metadata service (IPv4, IPv6)",
    Optional:            true,
},
```

#### 4. Advanced AWS Configuration
```go
// Add to TconsAwsProviderModel
StsRegion           types.String `tfsdk:"sts_region"`
UseDualStackEndpoint types.Bool   `tfsdk:"use_dualstack_endpoint"`
UseFIPSEndpoint     types.Bool   `tfsdk:"use_fips_endpoint"`
```

**Schema additions needed:**
```go
"sts_region": schema.StringAttribute{
    MarkdownDescription: "AWS region for STS operations (overrides default region)",
    Optional:            true,
},
"use_dualstack_endpoint": schema.BoolAttribute{
    MarkdownDescription: "Use AWS DualStack endpoints for IPv4/IPv6 support",
    Optional:            true,
},
"use_fips_endpoint": schema.BoolAttribute{
    MarkdownDescription: "Use FIPS 140-2 validated endpoints",
    Optional:            true,
},
```

#### 5. Missing Schema for Existing Model Fields
These fields exist in `TconsAwsProviderModel` but are missing from the schema:

```go
// Already in model, need to add to schema
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
```

## Missing Configure Function Logic

### 1. Shared Credentials/Config Files Handling
**Current:** Not implemented
**Required:**
```go
// Handle shared credentials files
if !data.SharedCredentialsFiles.IsNull() && !data.SharedCredentialsFiles.IsUnknown() {
    var files []string
    data.SharedCredentialsFiles.ElementsAs(ctx, &files, false)
    configOptions = append(configOptions, config.WithSharedCredentialsFiles(files))
}

// Handle shared config files  
if !data.SharedConfigFiles.IsNull() && !data.SharedConfigFiles.IsUnknown() {
    var files []string
    data.SharedConfigFiles.ElementsAs(ctx, &files, false)
    configOptions = append(configOptions, config.WithSharedConfigFiles(files))
}
```

### 2. Account ID Validation
**Current:** Not implemented
**Required:**
```go
// After loading config, validate account ID restrictions
if !data.AllowedAccountIds.IsNull() || !data.ForbiddenAccountIds.IsNull() {
    stsClient := sts.NewFromConfig(cfg)
    identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
    if err != nil && !data.SkipRequestingAccountId.ValueBool() {
        resp.Diagnostics.AddError("Failed to get caller identity", err.Error())
        return
    }
    
    if identity != nil && identity.Account != nil {
        accountId := *identity.Account
        
        // Check allowed list
        if !data.AllowedAccountIds.IsNull() {
            var allowed []string
            data.AllowedAccountIds.ElementsAs(ctx, &allowed, false)
            if len(allowed) > 0 && !contains(allowed, accountId) {
                resp.Diagnostics.AddError("Account not allowed", 
                    fmt.Sprintf("Account %s is not in allowed_account_ids", accountId))
                return
            }
        }
        
        // Check forbidden list
        if !data.ForbiddenAccountIds.IsNull() {
            var forbidden []string
            data.ForbiddenAccountIds.ElementsAs(ctx, &forbidden, false)
            if contains(forbidden, accountId) {
                resp.Diagnostics.AddError("Account forbidden",
                    fmt.Sprintf("Account %s is in forbidden_account_ids", accountId))
                return
            }
        }
    }
}
```

### 3. HTTP/Proxy Configuration
**Current:** Not implemented
**Required:**
```go
// Configure HTTP client with proxy and TLS settings
if !data.HttpProxy.IsNull() || !data.HttpsProxy.IsNull() || !data.Insecure.ValueBool() || !data.CustomCABundle.IsNull() {
    transport := &http.Transport{}
    
    // Proxy configuration
    if !data.HttpProxy.IsNull() || !data.HttpsProxy.IsNull() {
        proxyURL := &url.URL{}
        if !data.HttpsProxy.IsNull() {
            proxyURL, _ = url.Parse(data.HttpsProxy.ValueString())
        } else if !data.HttpProxy.IsNull() {
            proxyURL, _ = url.Parse(data.HttpProxy.ValueString())
        }
        transport.Proxy = http.ProxyURL(proxyURL)
    }
    
    // TLS configuration
    if data.Insecure.ValueBool() || !data.CustomCABundle.IsNull() {
        tlsConfig := &tls.Config{}
        if data.Insecure.ValueBool() {
            tlsConfig.InsecureSkipVerify = true
        }
        if !data.CustomCABundle.IsNull() {
            // Load custom CA bundle
            caCert, err := ioutil.ReadFile(data.CustomCABundle.ValueString())
            if err != nil {
                resp.Diagnostics.AddError("Failed to load CA bundle", err.Error())
                return
            }
            caCertPool := x509.NewCertPool()
            caCertPool.AppendCertsFromPEM(caCert)
            tlsConfig.RootCAs = caCertPool
        }
        transport.TLSClientConfig = tlsConfig
    }
    
    httpClient := &http.Client{Transport: transport}
    configOptions = append(configOptions, config.WithHTTPClient(httpClient))
}
```

### 4. Enhanced Retry Configuration
**Current:** Only max_retries is handled
**Required:**
```go
// Handle retry mode in addition to max_retries
if !data.RetryMode.IsNull() && !data.RetryMode.IsUnknown() {
    switch data.RetryMode.ValueString() {
    case "legacy":
        configOptions = append(configOptions, config.WithRetryMode(aws.RetryModeLegacy))
    case "standard":
        configOptions = append(configOptions, config.WithRetryMode(aws.RetryModeStandard))
    case "adaptive":
        configOptions = append(configOptions, config.WithRetryMode(aws.RetryModeAdaptive))
    default:
        resp.Diagnostics.AddError("Invalid retry mode", 
            "retry_mode must be one of: legacy, standard, adaptive")
        return
    }
}

if !data.MaxRetries.IsNull() && !data.MaxRetries.IsUnknown() {
    configOptions = append(configOptions, config.WithRetryMaxAttempts(int(data.MaxRetries.ValueInt64())))
}
```

### 5. IMDS Configuration
**Current:** Not implemented
**Required:**
```go
// Configure EC2 metadata service
if !data.EC2MetadataServiceEnableState.IsNull() {
    switch data.EC2MetadataServiceEnableState.ValueString() {
    case "enabled":
        configOptions = append(configOptions, config.WithEC2IMDSClientEnableState(imds.ClientEnabled))
    case "disabled":
        configOptions = append(configOptions, config.WithEC2IMDSClientEnableState(imds.ClientDisabled))
    }
}

if !data.EC2MetadataServiceEndpoint.IsNull() {
    configOptions = append(configOptions, config.WithEC2IMDSEndpoint(data.EC2MetadataServiceEndpoint.ValueString()))
}
```

### 6. FIPS/DualStack Endpoint Configuration
**Current:** Not implemented
**Required:**
```go
// Configure endpoint options
if data.UseFIPSEndpoint.ValueBool() {
    configOptions = append(configOptions, config.WithUseFIPSEndpoint(aws.FIPSEndpointStateEnabled))
}

if data.UseDualStackEndpoint.ValueBool() {
    configOptions = append(configOptions, config.WithUseDualStackEndpoint(aws.DualStackEndpointStateEnabled))
}
```

## Implementation Priority

### High Priority (Critical for AWS parity)
1. ✅ Shared credentials/config files schema and handling
2. ✅ Account ID validation (allowed/forbidden lists)
3. ✅ Enhanced retry configuration (retry_mode)
4. ✅ Skip credentials validation option

### Medium Priority (Important for enterprise environments)
1. ✅ HTTP/Proxy configuration
2. ✅ Custom CA bundle support
3. ✅ STS region override
4. ✅ IMDS configuration

### Lower Priority (Advanced/specialized use cases)
1. ✅ FIPS endpoint support
2. ✅ DualStack endpoint support
3. ✅ Insecure TLS option

## Testing Requirements

After implementing these features, the following tests should be added:

1. **Account validation tests**
   - Test allowed_account_ids enforcement
   - Test forbidden_account_ids enforcement
   - Test skip_requesting_account_id behavior

2. **Proxy configuration tests**
   - Test HTTP/HTTPS proxy usage
   - Test no_proxy bypass
   - Test custom CA bundle loading

3. **Enhanced retry tests**
   - Test different retry modes (legacy, standard, adaptive)
   - Test combined max_retries and retry_mode

4. **IMDS configuration tests**
   - Test IMDS enable/disable states
   - Test custom IMDS endpoint

## Migration Considerations

- All new attributes should be optional to maintain backward compatibility
- Default values should match AWS SDK defaults where applicable
- Environment variable support should follow AWS provider conventions
- Documentation should be updated to reflect new configuration options

## Next Steps

1. Update provider schema with missing attributes
2. Update TconsAwsProviderModel with missing fields
3. Implement Configure function enhancements
4. Add comprehensive tests
5. Update provider documentation