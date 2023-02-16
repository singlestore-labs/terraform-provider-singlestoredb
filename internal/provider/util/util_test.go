package util_test

import (
	"testing"

	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestDerefInt(t *testing.T) {
	var i *int
	result := util.Deref(i)
	require.Equal(t, 0, result)

	value := 10
	i = &value
	result = util.Deref(i)
	require.Equal(t, value, result)
}

func TestDerefSlice(t *testing.T) {
	var s *[]int
	result := util.Deref(s)
	require.Equal(t, []int(nil), result)

	value := []int{1, 2, 3}
	s = &value

	result = util.Deref(s)
	require.Equal(t, value, result)
}
