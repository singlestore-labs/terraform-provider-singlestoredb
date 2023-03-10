package testutil

import "strings"

// UpdatableConfig is the convenience for updating the config example for tests.
type UpdatableConfig string

// WithOverride replaces k with v.
func (uc UpdatableConfig) WithOverride(k, v string) UpdatableConfig {
	return UpdatableConfig(strings.ReplaceAll(uc.String(), k, v))
}

// String shows the result.
func (uc UpdatableConfig) String() string {
	return string(uc)
}
