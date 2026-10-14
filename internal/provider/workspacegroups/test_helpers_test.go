package workspacegroups_test

import (
	"net/http"
	"strings"
	"time"
)

func isMethodPath(r *http.Request, method, path string) bool {
	return r.Method == method && r.URL.Path == path
}

func isMethodSuffix(r *http.Request, method, suffix string) bool {
	return r.Method == method && strings.HasSuffix(r.URL.Path, suffix)
}

func mustParseTimePtr(s string) *time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
	}
	if err != nil {
		panic(err)
	}

	return &t
}
