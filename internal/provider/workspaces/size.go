package workspaces

import (
	"math"
	"strconv"
	"strings"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const delta = 1e-6

type Size struct {
	suspended bool
	decimal   float64
}

// ParseSize parses the response of the Management API.
//
// The response of the management API never returns a zero size.
func ParseSize(value string) (Size, *util.SummaryWithDetailError) {
	decimal := 0.0

	if strings.HasPrefix(value, "S-") {
		switch value {
		case "S-00":
			decimal = 0.25
		case "S-0":
			decimal = 0.5
		case "S-1":
			decimal = 1.0
		default:
			// Parse the integer value following.
			i, err := strconv.ParseInt(value[2:], 10, 64)
			if err != nil {
				return Size{}, &util.SummaryWithDetailError{
					Summary: config.CreateProviderIssueErrorDetail,
					Detail:  err.Error(),
				}
			}

			decimal = float64(i)
		}
	} else {
		// Parse as a float.
		var err error
		decimal, err = strconv.ParseFloat(value, 64)
		if err != nil {
			return Size{}, &util.SummaryWithDetailError{
				Summary: config.CreateProviderIssueErrorDetail,
				Detail:  err.Error(),
			}
		}
	}

	return Size{
		suspended: false,
		decimal:   decimal,
	}, nil
}

func (ws Size) String() string {
	if ws.decimal == 0.0 {
		return "0"
	}

	s := strconv.FormatFloat(ws.decimal, 'f', -1, 64)

	return strings.TrimRight(strings.TrimRight(s, "0"), ".")
}

func (ws Size) Eq(rhs Size) bool {
	if ws.suspended && rhs.suspended {
		return true
	}

	return math.Abs(ws.decimal-rhs.decimal) <= delta
}
