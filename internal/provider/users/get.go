package users

import (
	"context"
	"fmt"

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
		MarkdownDescription: "Retrieve a specific user using its ID or email with this data source.",
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

	if data.ID.IsNull() && data.Email.IsNull() {
		resp.Diagnostics.AddError(
			"Missing required attribute",
			"Either the ID or email attribute must be set to retrieve a user.",
		)

		return
	}
	var result UserModel
	if !data.ID.IsNull() {
		result = d.fundUserByID(ctx, data.ID.ValueString(), resp)
	} else if !data.Email.IsNull() {
		result = d.findUserByEmail(ctx, data.Email.ValueString(), resp)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (d *userDataSourceGet) fundUserByID(ctx context.Context, idStr string, resp *datasource.ReadResponse) UserModel {
	id, err := uuid.Parse(idStr)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.IDAttribute),
			"Invalid user ID",
			"The user ID should be a valid UUID",
		)

		return UserModel{}
	}

	user, err := d.GetV1betaUsersUserIDWithResponse(ctx, id, &management.GetV1betaUsersUserIDParams{})
	if serr := util.StatusOK(user, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return UserModel{}
	}

	return toUserModel(*user.JSON200)
}

func (d *userDataSourceGet) findUserByEmail(ctx context.Context, email string, resp *datasource.ReadResponse) UserModel {
	users, err := d.GetV1betaUsersWithResponse(ctx, &management.GetV1betaUsersParams{Email: &email})
	if serr := util.StatusOK(users, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return UserModel{}
	}

	if users.JSON200 == nil || len(*users.JSON200) < 1 {
		resp.Diagnostics.AddError(
			"User not found",
			fmt.Sprintf("User with the specified email %s does not exist.", email),
		)

		return UserModel{}
	}

	return toUserModel((*users.JSON200)[0])
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
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "The unique identifier of the user.",
			Validators:          []validator.String{util.NewUUIDValidator()},
		},
		"email": schema.StringAttribute{
			Optional:            true,
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
