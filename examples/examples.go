package examples

import "embed"

//go:embed */main.tf */datasource/main.tf */resource/main.tf
var f embed.FS

var (
	Provider                  = mustRead("provider/main.tf")
	Regions                   = mustRead("regions/main.tf")
	WorkspaceGroupsDataSource = mustRead("workspacegroups/datasource/main.tf")
	WorkspaceGroupsResource   = mustRead("workspacegroups/resource/main.tf")
)

func mustRead(path string) string {
	result, err := f.ReadFile(path)
	if err != nil {
		panic(err)
	}

	return string(result)
}
