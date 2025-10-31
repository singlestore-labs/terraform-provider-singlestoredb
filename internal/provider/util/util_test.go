package util_test

import (
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestSummaryWithDetailError(t *testing.T) {
	summary := "summary"
	detail := "detail"
	err := &util.SummaryWithDetailError{Summary: summary, Detail: detail}
	require.ErrorContains(t, err, summary)
	require.ErrorContains(t, err, detail)
}

func TestTerraformProviderUserAgent(t *testing.T) {
	version := "1.0.0"
	agent := util.TerraformProviderUserAgent(version)
	require.Contains(t, agent, version)
}

func TestDataSourceTypeName(t *testing.T) {
	name := util.DataSourceTypeName(datasource.MetadataRequest{ProviderTypeName: "foo"}, "bar")
	require.Equal(t, "foo_bar", name)
}

func TestResourceTypeName(t *testing.T) {
	name := util.ResourceTypeName(resource.MetadataRequest{ProviderTypeName: "foo"}, "bar")
	require.Equal(t, "foo_bar", name)
}

func TestPtr(t *testing.T) {
	s := "foo"
	p := util.Ptr(s)
	require.NotNil(t, p)
	require.Equal(t, s, *p)
}

func TestFirstNotEmpty(t *testing.T) {
	require.Equal(t, util.FirstNotEmpty(), "")
	require.Equal(t, util.FirstNotEmpty("a"), "a")
	require.Equal(t, util.FirstNotEmpty("", "a"), "a")
	require.Equal(t, util.FirstNotEmpty("a", "b"), "a")
}

func TestFirstSetStringValue(t *testing.T) {
	require.Equal(t, util.FirstSetStringValue(), types.StringNull())
	require.Equal(t, util.FirstSetStringValue(types.StringNull()), types.StringNull())
	require.Equal(t, util.FirstSetStringValue(types.StringUnknown(), types.StringNull()), types.StringNull())
	require.Equal(t, util.FirstSetStringValue(types.StringUnknown(), types.StringValue("foo"), types.StringNull()).ValueString(), "foo")
}

func TestMapWithError(t *testing.T) {
	{
		result, err := util.MapWithError([]string{}, func(_ string) (int, *util.SummaryWithDetailError) { return 0, nil })
		require.Nil(t, err)
		require.Empty(t, result)
	}
	{
		result, err := util.MapWithError([]int{1, 2}, func(i int) (string, *util.SummaryWithDetailError) { return strconv.Itoa(i), nil })
		require.Nil(t, err)
		require.Len(t, result, 2)
		require.Equal(t, result[0], "1")
		require.Equal(t, result[1], "2")
	}
	{
		_, err := util.MapWithError([]string{"a"}, func(_ string) (int, *util.SummaryWithDetailError) { return 0, &util.SummaryWithDetailError{} })
		require.NotNil(t, err)
	}
}

func TestCheckLastN(t *testing.T) {
	require.False(t, util.CheckLastN([]string{}, 10, "foo"))
	require.True(t, util.CheckLastN([]string{}, 0, "foo"))
	require.False(t, util.CheckLastN([]string{"bar"}, 1, "foo"))
	require.True(t, util.CheckLastN([]string{"foo"}, 1, "foo"))
	require.False(t, util.CheckLastN([]string{"foo", "bar"}, 1, "foo"))
	require.True(t, util.CheckLastN([]string{"foo", "bar"}, 1, "foo", "bar"))
	require.False(t, util.CheckLastN([]string{"bar", "foo"}, 2, "foo"))
	require.True(t, util.CheckLastN([]string{"bar", "foo", "foo"}, 2, "foo"))
	require.False(t, util.CheckLastN([]string{"foo", "bar", "foo"}, 2, "foo"))
}

func TestReadNotEmptyFileTrimmed(t *testing.T) {
	_, err := util.ReadNotEmptyFileTrimmed("/no/such/path/for/sure.txt")
	require.Error(t, err)

	_, err = util.ReadNotEmptyFileTrimmed("not/absolute/path.txt")
	require.Error(t, err)

	path, clean, err := testutil.CreateTemp("")
	require.NoError(t, err)
	t.Cleanup(clean)
	_, err = util.ReadNotEmptyFileTrimmed(path)
	require.Error(t, err, "because the file is empty")

	path, clean, err = testutil.CreateTemp("\nfoo ")
	require.NoError(t, err)
	t.Cleanup(clean)
	result, err := util.ReadNotEmptyFileTrimmed(path)
	require.NoError(t, err)
	require.Equal(t, "foo", result)
}

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

func TestJoin(t *testing.T) {
	result := util.Join([]management.WorkspaceState{management.WorkspaceStateACTIVE, management.WorkspaceStateSUSPENDED}, ", ")
	require.Equal(t, result, "ACTIVE, SUSPENDED")
}

func TestFilter(t *testing.T) {
	nums := []int{1, 2, 3, 4, 5, 6}
	evenNums := util.Filter(nums, func(n int) bool {
		return n%2 == 0
	})

	require.Equal(t, []int{2, 4, 6}, evenNums)
}
