package projects

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// projectListItem maps project fields shared by the single-project and list data sources.
type projectListItem struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Edition   types.String `tfsdk:"edition"`
	CreatedAt types.String `tfsdk:"created_at"`
}

type projectItemSchemaConfig struct {
	computeProjectID    bool
	requiredProjectID   bool
	projectIDValidators []validator.String
}

func newProjectItemSchemaAttributes(conf projectItemSchemaConfig) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		config.IDAttribute: schema.StringAttribute{
			Computed:            conf.computeProjectID,
			Required:            conf.requiredProjectID,
			MarkdownDescription: "The unique identifier of the project.",
			Validators:          conf.projectIDValidators,
		},
		"name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The name of the project.",
		},
		"edition": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The edition of the project.",
		},
		"created_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The timestamp when the project was created.",
		},
	}
}

func toProjectListItem(project management.Project) projectListItem {
	return projectListItem{
		ID:        util.UUIDStringValue(project.ProjectID),
		Name:      types.StringValue(project.Name),
		Edition:   types.StringValue(string(project.Edition)),
		CreatedAt: types.StringValue(project.CreatedAt.String()),
	}
}
