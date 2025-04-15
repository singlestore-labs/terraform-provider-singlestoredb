package users

import (
	"context"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	DataSourceGetName = "user"
)

// userDataSourceGet is the data source implementation.
type userDataSourceGet struct {
	management.ClientWithResponsesInterface
}

var _ datasource.DataSourceWithConfigure = &userDataSourceGet{}

// NewDataSourceGet is a helper function to simplify the provider implementation.
func NewDataSourceGet() datasource.DataSource {
	return &userDataSourceGet{}
}

// Metadata returns the data source type name.
func (d *userDataSourceGet) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceGetName)
}

// Schema defines the schema for the data source.
func (d *userDataSourceGet) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieve a specific user using its ID with this data source.",
		Attributes:          newUserDataSourceSchemaAttributes(),
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *userDataSourceGet) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.IDAttribute),
			"Invalid user ID",
			"The user ID should be a valid UUID",
		)

		return
	}

	user, err := d.GetV1betaUsersUserIDWithResponse(ctx, id, &management.GetV1betaUsersUserIDParams{})
	if serr := util.StatusOK(user, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	result, terr := toUserModel(*user.JSON200)
	if terr != nil {
		resp.Diagnostics.AddError(terr.Summary, terr.Detail)

		return
	}

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *userDataSourceGet) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func newUserDataSourceSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		config.IDAttribute: schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "The unique identifier of the user.",
			Validators:          []validator.String{util.NewUUIDValidator()},
		},
		"email": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The email address of the user.",
		},
		"first_name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "First name of the user.",
		},
		"last_name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Last name of the user.",
		},
	}
}
