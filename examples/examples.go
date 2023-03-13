package examples

import "embed"

//go:embed */main.tf */list/main.tf */get/main.tf */resource/main.tf
var f embed.FS

var (
	Provider                      = mustRead("provider/main.tf")
	Regions                       = mustRead("regions/main.tf")
	WorkspaceGroupsListDataSource = mustRead("workspacegroups/list/main.tf")
	WorkspaceGroupsGetDataSource  = mustRead("workspacegroups/get/main.tf")
	WorkspaceGroupsResource       = mustRead("workspacegroups/resource/main.tf")
)

func mustRead(path string) string {
	result, err := f.ReadFile(path)
	if err != nil {
		panic(err)
	}

	return string(result)
}
