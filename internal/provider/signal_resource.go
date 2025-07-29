// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	frameworkTypes "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &SignalResource{}
var _ resource.ResourceWithImportState = &SignalResource{}

func NewSignalResource() resource.Resource {
	return &SignalResource{}
}

// SignalResource defines the resource implementation.
type SignalResource struct {
	providerData *ProviderData
}

// SignalResourceModel describes the resource data model.
type SignalResourceModel struct {
	QueueURL       frameworkTypes.String `tfsdk:"queue_url"`
	SignalID       frameworkTypes.String `tfsdk:"signal_id"`
	ExpectedCount  frameworkTypes.Int64  `tfsdk:"expected_count"`
	Retries        frameworkTypes.Int64  `tfsdk:"retries"`
	PublishTimeout frameworkTypes.String `tfsdk:"publish_timeout"`
	Triggers       frameworkTypes.Map    `tfsdk:"triggers"`

	// Computed
	SuccessCount    frameworkTypes.Int64  `tfsdk:"success_count"`
	FailureReceived frameworkTypes.Bool   `tfsdk:"failure_received"`
	InstanceIDs     frameworkTypes.List   `tfsdk:"instance_ids"`
	ID              frameworkTypes.String `tfsdk:"id"`

	// Timeouts
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (r *SignalResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_signal"
}

func (r *SignalResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The `tconsaws_signal` resource waits for EC2 instances or other compute resources to send success/failure signals via SQS, providing CloudFormation cfn-signal equivalent functionality for Terraform.",

		Attributes: map[string]schema.Attribute{
			"queue_url": schema.StringAttribute{
				MarkdownDescription: "SQS queue URL or ARN where signals will be sent",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"signal_id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for this deployment/signal group. Messages must have this as a message attribute to be counted.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"expected_count": schema.Int64Attribute{
				MarkdownDescription: "Number of success signals required before considering the resource complete",
				Required:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"retries": schema.Int64Attribute{
				MarkdownDescription: "Number of retries for transient SQS errors",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(3),
			},
			"publish_timeout": schema.StringAttribute{
				MarkdownDescription: "Timeout duration for each SQS SendMessage call (e.g., '10s', '1m')",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("10s"),
			},
			"triggers": schema.MapAttribute{
				MarkdownDescription: "Map of arbitrary strings that, when changed, will force recreation of the resource",
				Optional:            true,
				ElementType:         frameworkTypes.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},

			// Computed attributes
			"success_count": schema.Int64Attribute{
				MarkdownDescription: "Number of success signals received",
				Computed:            true,
			},
			"failure_received": schema.BoolAttribute{
				MarkdownDescription: "Whether any failure signal was received",
				Computed:            true,
			},
			"instance_ids": schema.ListAttribute{
				MarkdownDescription: "List of unique instance IDs that sent signals",
				Computed:            true,
				ElementType:         frameworkTypes.StringType,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Resource identifier",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},

		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Delete: true,
			}),
		},
	}
}

func (r *SignalResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*ProviderData)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.providerData = providerData
}

func (r *SignalResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SignalResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Handle timeout from configuration
	createTimeout, diags := data.Timeouts.Create(ctx, 30*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	tflog.Info(ctx, "Starting SQS signal polling", map[string]interface{}{
		"queue_url":      data.QueueURL.ValueString(),
		"signal_id":      data.SignalID.ValueString(),
		"expected_count": data.ExpectedCount.ValueInt64(),
		"timeout":        createTimeout.String(),
	})

	// Perform SQS polling (to be implemented in next step)
	err := r.pollForSignals(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError(
			"Signal polling failed",
			fmt.Sprintf("Error while waiting for signals: %s", err.Error()),
		)
		return
	}

	// Generate ID
	data.ID = frameworkTypes.StringValue(fmt.Sprintf("%s:%s", data.SignalID.ValueString(), data.QueueURL.ValueString()))

	tflog.Info(ctx, "Signal polling completed successfully")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SignalResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SignalResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// No-op: signal resource has no persistent external state to refresh
	// The resource state reflects the completed polling operation

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SignalResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All schema changes are marked as RequiresReplace, so this should never be called
	resp.Diagnostics.AddError(
		"Update not supported",
		"All changes to tconsaws_signal resource require replacement. This should not happen.",
	)
}

func (r *SignalResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SignalResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// No external cleanup needed - SQS queue is managed externally
	// Just log the deletion
	tflog.Info(ctx, "Signal resource deleted", map[string]interface{}{
		"signal_id": data.SignalID.ValueString(),
	})
}

func (r *SignalResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// pollForSignals implements the core SQS polling logic.
func (r *SignalResource) pollForSignals(ctx context.Context, data *SignalResourceModel) error {
	queueURL := data.QueueURL.ValueString()
	signalID := data.SignalID.ValueString()
	expectedCount := data.ExpectedCount.ValueInt64()

	tflog.Debug(ctx, "Polling SQS for signals", map[string]interface{}{
		"queue_url":      queueURL,
		"signal_id":      signalID,
		"expected_count": expectedCount,
	})

	// Track unique instance IDs that have sent success signals
	successfulInstances := make(map[string]bool)
	var instanceIDsList []string
	var successCount int64 = 0
	var failureReceived bool

	// Polling loop
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for signals: %w", ctx.Err())
		default:
			// Continue polling
		}

		// Receive messages from SQS with long polling
		input := &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			WaitTimeSeconds:     20, // Long polling
			MaxNumberOfMessages: 10, // Process up to 10 messages at once
			MessageAttributeNames: []string{
				"All", // Get all message attributes
			},
		}

		result, err := r.providerData.SQSClient.ReceiveMessage(ctx, input)
		if err != nil {
			tflog.Error(ctx, "Error receiving messages from SQS", map[string]interface{}{
				"error": err.Error(),
			})
			// Continue polling on transient errors
			continue
		}

		// Process each message
		for _, message := range result.Messages {
			processed, err := r.processMessage(ctx, message, signalID, successfulInstances)
			if err != nil {
				tflog.Warn(ctx, "Error processing message", map[string]interface{}{
					"error":      err.Error(),
					"message_id": aws.ToString(message.MessageId),
				})
				continue
			}

			if processed {
				// Delete the processed message
				deleteInput := &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(queueURL),
					ReceiptHandle: message.ReceiptHandle,
				}
				_, deleteErr := r.providerData.SQSClient.DeleteMessage(ctx, deleteInput)
				if deleteErr != nil {
					tflog.Warn(ctx, "Failed to delete processed message", map[string]interface{}{
						"error":      deleteErr.Error(),
						"message_id": aws.ToString(message.MessageId),
					})
				}

				// Check if this was a failure signal
				if status, exists := message.MessageAttributes["status"]; exists && status.StringValue != nil {
					if strings.ToUpper(*status.StringValue) == "FAILURE" {
						tflog.Error(ctx, "Received failure signal", map[string]interface{}{
							"signal_id": signalID,
						})
						return fmt.Errorf("received failure signal from instance")
					}
				}
			}
		}

		// Update counts
		successCount = int64(len(successfulInstances))
		instanceIDsList = make([]string, 0, len(successfulInstances))
		for instanceID := range successfulInstances {
			instanceIDsList = append(instanceIDsList, instanceID)
		}

		tflog.Debug(ctx, "Signal polling status", map[string]interface{}{
			"success_count":  successCount,
			"expected_count": expectedCount,
			"instance_ids":   instanceIDsList,
		})

		// Check if we have enough success signals
		if successCount >= expectedCount {
			tflog.Info(ctx, "Received expected number of success signals", map[string]interface{}{
				"success_count":  successCount,
				"expected_count": expectedCount,
				"instance_ids":   instanceIDsList,
			})
			break
		}
	}

	// Update the data model with results
	data.SuccessCount = frameworkTypes.Int64Value(successCount)
	data.FailureReceived = frameworkTypes.BoolValue(failureReceived)

	// Convert instance IDs to framework list
	instanceValues := make([]attr.Value, len(instanceIDsList))
	for i, id := range instanceIDsList {
		instanceValues[i] = frameworkTypes.StringValue(id)
	}
	data.InstanceIDs = frameworkTypes.ListValueMust(frameworkTypes.StringType, instanceValues)

	return nil
}

// processMessage processes a single SQS message and returns true if it was relevant to our signal.
func (r *SignalResource) processMessage(ctx context.Context, message types.Message, signalID string, successfulInstances map[string]bool) (bool, error) {
	// Check if message has the required signal_id attribute
	signalIDAttr, hasSignalID := message.MessageAttributes["signal_id"]
	if !hasSignalID || signalIDAttr.StringValue == nil {
		// Not our message, ignore
		return false, nil
	}

	if *signalIDAttr.StringValue != signalID {
		// Different signal ID, ignore
		return false, nil
	}

	// Get instance ID
	instanceIDAttr, hasInstanceID := message.MessageAttributes["instance_id"]
	if !hasInstanceID || instanceIDAttr.StringValue == nil {
		return false, fmt.Errorf("message missing instance_id attribute")
	}
	instanceID := *instanceIDAttr.StringValue

	// Get status
	statusAttr, hasStatus := message.MessageAttributes["status"]
	if !hasStatus || statusAttr.StringValue == nil {
		return false, fmt.Errorf("message missing status attribute")
	}
	status := strings.ToUpper(*statusAttr.StringValue)

	tflog.Debug(ctx, "Processing signal message", map[string]interface{}{
		"signal_id":   signalID,
		"instance_id": instanceID,
		"status":      status,
		"message_id":  aws.ToString(message.MessageId),
	})

	// Process based on status
	switch status {
	case "SUCCESS":
		// Mark this instance as successful (prevents duplicates)
		successfulInstances[instanceID] = true
		tflog.Info(ctx, "Received success signal", map[string]interface{}{
			"signal_id":   signalID,
			"instance_id": instanceID,
		})
		return true, nil
	case "FAILURE":
		// Failure signals are processed but don't add to success count
		tflog.Error(ctx, "Received failure signal", map[string]interface{}{
			"signal_id":   signalID,
			"instance_id": instanceID,
		})
		return true, nil
	default:
		return false, fmt.Errorf("unknown status: %s", status)
	}
}
