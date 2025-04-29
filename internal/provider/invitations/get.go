package invitations

import (
	"context"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	DataSourceGetName = "invitation"
)

// InvitationModel maps the resource schema data.
type InvitationModel struct {
	ID        types.String   `tfsdk:"id"`
	Email     types.String   `tfsdk:"email"`
	State     types.String   `tfsdk:"state"`
	Teams     []types.String `tfsdk:"teams"`
	CreatedAt types.String   `tfsdk:"created_at"`
}

// invitationDataSourceGet is the data source implementation.
type invitationDataSourceGet struct {
	management.ClientWithResponsesInterface
}

var _ datasource.DataSourceWithConfigure = &invitationDataSourceGet{}

// NewDataSourceGet is a helper function to simplify the provider implementation.
func NewDataSourceGet() datasource.DataSource {
	return &invitationDataSourceGet{}
}

// Metadata returns the data source type name.
func (d *invitationDataSourceGet) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceGetName)
}

// Schema defines the schema for the data source.
func (d *invitationDataSourceGet) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieve a specific user invitation using its ID with this data source.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique identifier of the invitation.",
				Validators:          []validator.String{util.NewUUIDValidator()},
			},
			"email": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The email address of the user.",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The state of the invitation. Possible values are Pending, Accepted, Refused, or Revoked.",
			},
			"teams": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "A list of user teams.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The timestamp when the invitation was created, in ISO 8601 format.",
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *invitationDataSourceGet) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data InvitationModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.IDAttribute),
			"Invalid invitation ID",
			"The invitation ID must be a valid UUID",
		)

		return
	}

	userInvitation, err := d.GetV1betaInvitationsInvitationIDWithResponse(ctx, id, &management.GetV1betaInvitationsInvitationIDParams{})
	if serr := util.StatusOK(userInvitation, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	result := toInvitationModel(*userInvitation.JSON200)

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *invitationDataSourceGet) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func toInvitationModel(userInvitation management.UserInvitation) InvitationModel {
	return InvitationModel{
		ID:        util.MaybeUUIDStringValue(userInvitation.InvitationID),
		Email:     util.MaybeStringValue(userInvitation.Email),
		State:     util.StringValueOrNull(userInvitation.State),
		Teams:     util.MaybeUUIDStringListValue(userInvitation.TeamIDs),
		CreatedAt: util.MaybeTimeValue(userInvitation.CreatedAt),
	}
}
