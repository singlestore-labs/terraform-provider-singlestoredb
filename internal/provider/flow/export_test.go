package flow

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
)

func FlowFieldAvailableForTest(s *string) bool {
	return flowFieldAvailable(s)
}

type FlowInstanceModelSnapshot struct {
	UserName     string
	DatabaseName string
	Endpoint     string
	UserNameSet  bool
	DatabaseSet  bool
}

func ToFlowInstanceResourceModelForTest(flow management.Flow, priorUserName, priorDatabaseName *string) FlowInstanceModelSnapshot {
	var prior *flowInstanceResourceModel
	if priorUserName != nil || priorDatabaseName != nil {
		prior = &flowInstanceResourceModel{}
		if priorUserName != nil {
			prior.UserName = types.StringValue(*priorUserName)
		}

		if priorDatabaseName != nil {
			prior.DatabaseName = types.StringValue(*priorDatabaseName)
		}
	}

	model := toFlowInstanceResourceModel(flow, prior)

	snap := FlowInstanceModelSnapshot{
		Endpoint: model.Endpoint.ValueString(),
	}
	if !model.UserName.IsNull() {
		snap.UserNameSet = true
		snap.UserName = model.UserName.ValueString()
	}

	if !model.DatabaseName.IsNull() {
		snap.DatabaseSet = true
		snap.DatabaseName = model.DatabaseName.ValueString()
	}

	return snap
}
