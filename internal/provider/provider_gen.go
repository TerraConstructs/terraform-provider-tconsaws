// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// https://github.com/hashicorp/terraform-provider-aws/blob/v6.0.0/internal/provider/framework/provider_gen.go

package provider

import (
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func endpointsBlock() schema.SetNestedBlock {
	return schema.SetNestedBlock{
		Validators: []validator.Set{setvalidator.SizeAtMost(1)}, // at most one block
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				// for now we only allow SQS endpoint overriding
				"sqs": schema.StringAttribute{
					Optional:    true,
					Description: "Use this to override the default service endpoint URL",
				},
			},
		},
	}
}

type EndpointsModel struct {
	SQS types.String `tfsdk:"sqs"`
}

func expandEndpointsModel(endpoints map[string]string, ep EndpointsModel, resp *provider.ConfigureResponse) {
	if endpoints == nil {
		endpoints = make(map[string]string)
	}
	if !ep.SQS.IsNull() && !ep.SQS.IsUnknown() {
		sqsEndpoint := ep.SQS.ValueString()
		if sqsEndpoint != "" {
			endpoints[sqs.ServiceID] = sqsEndpoint
		} else {
			resp.Diagnostics.AddWarning(
				"Empty SQS Endpoint",
				"The SQS endpoint is empty, using default AWS SQS endpoint.",
			)
		}
	}
}
