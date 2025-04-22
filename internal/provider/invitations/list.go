package invitations

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	DataSourceListName = "invitations"
)

// invitationsDataSourceList is the data source implementation.
type invitationsDataSourceList struct {
	management.ClientWithResponsesInterface
}

// invitationsListDataSourceModel maps the data source schema data.
type invitationsListDataSourceModel struct {
	ID          types.String      `tfsdk:"id"`
	Email       types.String      `tfsdk:"email"`
	Invitations []InvitationModel `tfsdk:"invitations"`
}

var _ datasource.DataSourceWithConfigure = &invitationsDataSourceList{}

// NewDataSourceList is a helper function to simplify the provider implementation.
func NewDataSourceList() datasource.DataSource {
	return &invitationsDataSourceList{}
}

// Metadata returns the data source type name.
func (d *invitationsDataSourceList) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceListName)
}

// Schema defines the schema for the data source.
func (d *invitationsDataSourceList) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "This data source provides a list of user invitations to current organization.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed: true,
			},
			"email": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The email address to filter the list of user invitations for specific user.",
			},
			DataSourceListName: schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						config.IDAttribute: schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique identifier of the invitation.",
						},
						"email": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The email address of the user associated with the invitation.",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The state of the invitation. Possible values are Pending, Accepted, Refused, or Revoked.",
						},
						"teams": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "A list of teams associated with the user.",
						},
						"created_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The timestamp indicating when the invitation was created.",
						},
					},
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *invitationsDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data invitationsListDataSourceModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var email *string
	if !data.Email.IsNull() {
		email = util.MaybeString(data.Email)
	}

	invitations, err := d.GetV1betaInvitationsWithResponse(ctx, &management.GetV1betaInvitationsParams{Email: email})
	if serr := util.StatusOK(invitations, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}
	resultInvitations := util.Map(util.Deref(invitations.JSON200), toInvitationModel)

	result := invitationsListDataSourceModel{
		ID:          types.StringValue(config.TestIDValue),
		Email:       data.Email,
		Invitations: resultInvitations,
	}

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *invitationsDataSourceList) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}
