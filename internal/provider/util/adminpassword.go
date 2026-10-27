package util

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
)

const (
	adminPasswordMaxSequential = 2
	adminPasswordMaxRepeat     = 3
)

// SkipAdminPasswordValidationAttribute turns off the provider's admin_password
// rule check for a resource. It exists so that a change to the Management API
// password rules does not block users until the provider is updated.
const SkipAdminPasswordValidationAttribute = "skip_admin_password_validation"

// AdminPasswordRules describes the Management API /v2/clusters password rules
// that the provider checks.
func AdminPasswordRules() string {
	return fmt.Sprintf(
		"at least %d characters, an uppercase letter, a lowercase letter, a digit, and a special character, "+
			"at most %d consecutive sequential characters (such as \"ab\" or \"12\"), "+
			"and at most %d identical characters in a row",
		config.AdminPasswordMinLength,
		adminPasswordMaxSequential,
		adminPasswordMaxRepeat,
	)
}

// AdminPasswordMarkdown describes admin_password for resource schemas.
// subject is the resource noun, such as "cluster" or "workspace group".
func AdminPasswordMarkdown(subject string) string {
	return fmt.Sprintf(
		"The admin SQL user password for the %s. If omitted, Management API /v2/clusters generates a password and Terraform stores that generated value. "+
			"A configured password must have %s. "+
			"The provider checks these rules during plan; set `%s = true` to turn the check off if the API rules have changed. "+
			"If the API still does not accept the configured password on create, it generates a different one, so the provider deletes the new %s and fails, "+
			"rather than store a password that cannot log in.",
		subject,
		AdminPasswordRules(),
		SkipAdminPasswordValidationAttribute,
		subject,
	)
}

// AdminPasswordProblems reports Management API /v2/clusters rules the password fails.
// An empty result means the API will keep this password on create.
func AdminPasswordProblems(password string) []string {
	problems := characterClassProblems(password)
	if utf8.RuneCountInString(password) < config.AdminPasswordMinLength {
		problems = append(problems, fmt.Sprintf("must be at least %d characters", config.AdminPasswordMinLength))
	}
	if maxConsecutiveSequential(password) > adminPasswordMaxSequential {
		problems = append(problems, fmt.Sprintf("must not contain more than %d consecutive sequential characters", adminPasswordMaxSequential))
	}
	if maxConsecutiveRepeat(password) > adminPasswordMaxRepeat {
		problems = append(problems, fmt.Sprintf("must not contain more than %d identical characters in a row", adminPasswordMaxRepeat))
	}

	return problems
}

func characterClassProblems(password string) []string {
	upper, lower, digit, special := passwordCharacterClasses(password)

	var problems []string
	if !upper {
		problems = append(problems, "must include an uppercase letter (A-Z)")
	}
	if !lower {
		problems = append(problems, "must include a lowercase letter (a-z)")
	}
	if !digit {
		problems = append(problems, "must include a digit")
	}
	if !special {
		problems = append(problems, "must include a special character (not a letter or digit)")
	}

	return problems
}

func passwordCharacterClasses(password string) (bool, bool, bool, bool) {
	var upper, lower, digit, special bool
	for _, r := range password {
		switch {
		case isASCIIUpper(r):
			upper = true
		case isASCIILower(r):
			lower = true
		case isASCIIDigit(r):
			digit = true
		default:
			special = true
		}
	}

	return upper, lower, digit, special
}

func isASCIIUpper(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func isASCIILower(r rune) bool {
	return r >= 'a' && r <= 'z'
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// maxConsecutiveSequential is the longest run of characters whose code points
// step by 1 in one direction. "ab" and "21" are runs of 2. "abc" and "321" are
// runs of 3. A change of direction starts a new run.
func maxConsecutiveSequential(password string) int {
	runes := []rune(password)
	if len(runes) == 0 {
		return 0
	}

	maxRun := 1
	run := 1
	var direction int32
	for i := 1; i < len(runes); i++ {
		step := runes[i] - runes[i-1]
		if step != 1 && step != -1 {
			run = 1

			continue
		}
		if run > 1 && step == direction {
			run++
		} else {
			direction = step
			run = 2
		}
		if run > maxRun {
			maxRun = run
		}
	}

	return maxRun
}

func maxConsecutiveRepeat(password string) int {
	runes := []rune(password)
	if len(runes) == 0 {
		return 0
	}

	maxRun := 1
	run := 1
	for i := 1; i < len(runes); i++ {
		if runes[i] != runes[i-1] {
			run = 1

			continue
		}
		run++
		if run > maxRun {
			maxRun = run
		}
	}

	return maxRun
}

// AdminPasswordWasReplaced reports that POST /v2/clusters returned a generated
// password different from the one the configuration sent. An empty API value
// means the request password was accepted: the create response includes
// adminPassword only when the API generated one.
func AdminPasswordWasReplaced(configured, apiReturned string) bool {
	if configured == "" || apiReturned == "" {
		return false
	}

	return configured != apiReturned
}

// CheckCreatedAdminPassword reports a failed create call, or deletes a cluster
// whose create response replaced the configured password.
func CheckCreatedAdminPassword(ctx context.Context, c management.ClientWithResponsesInterface, resp *management.PostV2ClustersResponse, callErr error, configured string) *SummaryWithDetailError {
	if serr := StatusOK(resp, callErr); serr != nil {
		return serr
	}

	return RejectReplacedAdminPassword(
		ctx,
		c,
		resp.JSON200.ClusterID,
		configured,
		Deref(resp.JSON200.AdminPassword),
	)
}

// RejectReplacedAdminPassword deletes a cluster whose create response replaced
// the configured password, and returns an error that explains how to set a
// password the API accepts. The generated password is never included, because
// error output often ends up in CI logs.
func RejectReplacedAdminPassword(ctx context.Context, c management.ClientWithResponsesInterface, clusterID uuid.UUID, configured, apiReturned string) *SummaryWithDetailError {
	if !AdminPasswordWasReplaced(configured, apiReturned) {
		return nil
	}

	summary := "Admin password was rejected"
	cause := "Management API /v2/clusters did not accept the configured admin_password and generated a different password instead. " +
		"Terraform does not store a password that cannot log in."

	var outcome string
	switch {
	case clusterID == uuid.Nil:
		outcome = "The create response did not include a cluster ID, so the new cluster could not be deleted. " +
			"Delete it in the SingleStore Portal before you apply again."
	default:
		deleteResponse, err := c.DeleteV2ClustersClusterIDWithResponse(ctx, clusterID)
		if serr := StatusOK(deleteResponse, err, ReturnNilOnNotFound); serr != nil {
			outcome = fmt.Sprintf("The new cluster %s could not be deleted (%s). "+
				"Delete it in the SingleStore Portal before you apply again.", clusterID, serr.Detail)
		} else {
			outcome = fmt.Sprintf("The new cluster %s was deleted.", clusterID)
		}
	}

	return &SummaryWithDetailError{
		Summary: summary,
		Detail:  cause + " " + outcome + "\n\n" + adminPasswordFixInstructions(configured),
	}
}

func adminPasswordFixInstructions(configured string) string {
	known := "The configured password meets the rules the provider knows, so the API rules have likely changed. " +
		"Check the SingleStore documentation for the current password requirements."
	if problems := AdminPasswordProblems(configured); len(problems) > 0 {
		known = "The configured password fails these rules: it " + strings.Join(problems, "; ") + "."
	}

	return "To fix this, do one of the following, then apply again:\n" +
		"  - Set admin_password to a password with " + AdminPasswordRules() + ". " + known + "\n" +
		"  - Remove admin_password to let the API generate a password. Terraform stores the generated password in admin_password."
}

var _ validator.String = &adminPasswordValidator{}

type adminPasswordValidator struct{}

func (v adminPasswordValidator) Description(context.Context) string {
	return "must meet Management API /v2/clusters password rules unless " + SkipAdminPasswordValidationAttribute + " is true"
}

func (v adminPasswordValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v adminPasswordValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	var skip types.Bool
	response.Diagnostics.Append(request.Config.GetAttribute(ctx, path.Root(SkipAdminPasswordValidationAttribute), &skip)...)
	if response.Diagnostics.HasError() {
		return
	}
	// An unknown skip value may turn out true, so do not block the plan on it.
	if skip.IsUnknown() || skip.ValueBool() {
		return
	}

	problems := AdminPasswordProblems(request.ConfigValue.ValueString())
	if len(problems) == 0 {
		return
	}

	response.Diagnostics.AddAttributeError(
		request.Path,
		"Invalid admin password",
		"The password does not meet the Management API /v2/clusters password rules: it "+strings.Join(problems, "; ")+". "+
			"/v2/clusters would replace it with a generated password, and the provider would then delete the new cluster.\n\n"+
			"Set admin_password to a password with "+AdminPasswordRules()+", or remove admin_password to let the API generate one. "+
			"If the API rules have changed and this check is out of date, set "+SkipAdminPasswordValidationAttribute+" = true.",
	)
}

// NewAdminPasswordValidator rejects passwords that /v2/clusters would replace
// with a generated password. Null and unknown values are skipped, so an omitted
// admin_password still lets the API generate one. The resource must define the
// SkipAdminPasswordValidationAttribute bool attribute; when it is true, the
// check is skipped and the create response check is the only guard.
func NewAdminPasswordValidator() validator.String {
	return &adminPasswordValidator{}
}
