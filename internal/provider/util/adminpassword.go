package util

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
)

const (
	adminPasswordMaxSequential = 2
	adminPasswordMaxRepeat     = 3
)

// AdminPasswordMarkdown describes admin_password for resource schemas.
// subject is the resource noun, such as "cluster" or "workspace group".
func AdminPasswordMarkdown(subject string) string {
	return fmt.Sprintf(
		"The admin SQL user password for the %s. If omitted, Management API /v2/clusters generates a password and Terraform stores that generated value. A configured password must contain at least %d characters, an uppercase letter, a lowercase letter, a digit, and a special character. It may include at most %d consecutive sequential characters, such as \"ab\" or \"12\", and at most %d identical characters in a row. The provider rejects a password that fails these rules, because /v2/clusters would generate a different password on create and return it only in that response.",
		subject,
		config.AdminPasswordMinLength,
		adminPasswordMaxSequential,
		adminPasswordMaxRepeat,
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
// the configured password. The generated password is included only when the
// cluster cannot be deleted, because that response is the only time the API
// returns it.
func RejectReplacedAdminPassword(ctx context.Context, c management.ClientWithResponsesInterface, clusterID uuid.UUID, configured, apiReturned string) *SummaryWithDetailError {
	if !AdminPasswordWasReplaced(configured, apiReturned) {
		return nil
	}

	summary := "Admin password was not applied"
	detail := "Management API /v2/clusters did not keep the configured admin_password and returned a different generated password. " +
		"That generated password is not available on later reads. " +
		"The new cluster was deleted so Terraform would not store a password that cannot log in. " +
		"Set a password that meets the API rules, or omit admin_password."

	if clusterID == uuid.Nil {
		return &SummaryWithDetailError{
			Summary: summary,
			Detail:  detail + " The create response did not include a cluster ID, so the cluster was not deleted. Generated password: " + apiReturned,
		}
	}

	deleteResponse, err := c.DeleteV2ClustersClusterIDWithResponse(ctx, clusterID)
	if serr := StatusOK(deleteResponse, err, ReturnNilOnNotFound); serr != nil {
		return &SummaryWithDetailError{
			Summary: summary,
			Detail:  fmt.Sprintf("%s Cluster %s could not be deleted (%s). Generated password: %s", detail, clusterID, serr.Detail, apiReturned),
		}
	}

	return &SummaryWithDetailError{Summary: summary, Detail: detail}
}

var _ validator.String = &adminPasswordValidator{}

type adminPasswordValidator struct{}

func (v adminPasswordValidator) Description(context.Context) string {
	return "must meet Management API /v2/clusters password rules"
}

func (v adminPasswordValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v adminPasswordValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	problems := AdminPasswordProblems(request.ConfigValue.ValueString())
	if len(problems) == 0 {
		return
	}

	response.Diagnostics.AddAttributeError(
		request.Path,
		"Invalid admin password",
		"The password does not meet the Management API /v2/clusters rules: "+strings.Join(problems, "; ")+
			". /v2/clusters replaces a password that fails these rules and returns the generated password only on create. "+
			"Terraform would store the configured value, which cannot log in.",
	)
}

// NewAdminPasswordValidator rejects passwords that /v2/clusters would replace
// with a generated password. Null and unknown values are skipped, so an omitted
// admin_password still lets the API generate one.
func NewAdminPasswordValidator() validator.String {
	return &adminPasswordValidator{}
}
