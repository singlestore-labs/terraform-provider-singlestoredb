package examples

import "embed"

//go:embed provider/main.tf regions/main.tf
var f embed.FS

var (
	Provider = mustRead("provider/main.tf")
	Regions  = mustRead("regions/main.tf")
)

func mustRead(path string) string {
	result, err := f.ReadFile(path)
	if err != nil {
		panic(err)
	}

	return string(result)
}
