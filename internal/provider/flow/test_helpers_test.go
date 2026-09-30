package flow_test

import "time"

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
