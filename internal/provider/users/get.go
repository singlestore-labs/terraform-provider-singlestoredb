package users

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	DataSourceGetName = "user"
)

// userModel maps the resource schema data.
type UserDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Email     types.String `tfsdk:"email"`
	FirstName types.String `tfsdk:"first_name"`
	LastName  types.String `tfsdk:"last_name"`
}

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
	var data UserDataSourceModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if d.isMissingRequiredAttributes(data, resp) {
		return
	}

	if !data.ID.IsNull() && !data.ID.IsUnknown() {
		d.handleUserByID(ctx, data.ID.ValueString(), resp)
	}
	d.handleUserByEmail(ctx, data.Email.ValueString(), resp)
}

func (d *userDataSourceGet) isMissingRequiredAttributes(data UserDataSourceModel, resp *datasource.ReadResponse) bool {
	if (data.ID.IsNull() || data.ID.IsUnknown()) &&
		(data.Email.IsNull() || data.Email.IsUnknown()) {
		resp.Diagnostics.AddError(
			"Missing required attribute",
			"Either the ID or email attribute must be set to retrieve a user.",
		)

		return true
	}

	return false
}

func (d *userDataSourceGet) handleUserByID(ctx context.Context, idStr string, resp *datasource.ReadResponse) {
	result, serr := d.findUserByID(ctx, idStr)
	if serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}
	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (d *userDataSourceGet) handleUserByEmail(ctx context.Context, email string, resp *datasource.ReadResponse) {
	result, serr := d.findUserByEmail(ctx, email)
	if serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}
	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (d *userDataSourceGet) findUserByID(ctx context.Context, idStr string) (*UserDataSourceModel, *util.SummaryWithDetailError) {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, &util.SummaryWithDetailError{
			Summary: "Invalid user ID",
			Detail:  "The user ID must be a valid UUID",
		}
	}

	user, err := d.GetV1betaUsersUserIDWithResponse(ctx, id, &management.GetV1betaUsersUserIDParams{})
	if serr := util.StatusOK(user, err); serr != nil {
		return nil, serr
	}

	data := toUserDataSourceModel(*user.JSON200)

	return &data, nil
}

func (d *userDataSourceGet) findUserByEmail(ctx context.Context, email string) (*UserDataSourceModel, *util.SummaryWithDetailError) {
	users, err := d.GetV1betaUsersWithResponse(ctx, &management.GetV1betaUsersParams{Email: &email})
	if serr := util.StatusOK(users, err); serr != nil {
		return nil, serr
	}

	if users.JSON200 == nil || len(*users.JSON200) < 1 {
		return nil, &util.SummaryWithDetailError{
			Summary: "User not found",
			Detail:  fmt.Sprintf("User with the specified email %s does not exist.", email),
		}
	}
	data := toUserDataSourceModel((*users.JSON200)[0])

	return &data, nil
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
			MarkdownDescription: "The first name of the user.",
		},
		"last_name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The last name of the user.",
		},
	}
}

func toUserDataSourceModel(user management.User) UserDataSourceModel {
	return UserDataSourceModel{
		ID:        util.UUIDStringValue(user.UserID),
		Email:     types.StringValue(user.Email),
		FirstName: types.StringValue(user.FirstName),
		LastName:  types.StringValue(user.LastName),
	}
}
