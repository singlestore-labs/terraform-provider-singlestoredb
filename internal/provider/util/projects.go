package util

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/singlestore-labs/singlestore-go/management"
)

// ResolveProjectIDByName returns the ID of the only project named projectName.
// kind names the resource being assigned in the error message, for example "cluster".
func ResolveProjectIDByName(ctx context.Context, c management.ClientWithResponsesInterface, projectName, kind string) (*uuid.UUID, *SummaryWithDetailError) {
	projectsResponse, err := c.GetV2ProjectsWithResponse(ctx)
	if serr := StatusOK(projectsResponse, err); serr != nil {
		return nil, serr
	}

	var sameNameProjectIDs []uuid.UUID
	availableProjectNamesSet := make(map[string]struct{})
	for _, project := range Deref(projectsResponse.JSON200) {
		if project.Name == projectName {
			sameNameProjectIDs = append(sameNameProjectIDs, project.ProjectID)
		}

		availableProjectNamesSet[project.Name] = struct{}{}
	}

	if len(sameNameProjectIDs) == 0 {
		availableProjectNames := make([]string, 0, len(availableProjectNamesSet))
		for name := range availableProjectNamesSet {
			availableProjectNames = append(availableProjectNames, fmt.Sprintf("'%s'", name))
		}

		availableProjectsDetail := ""
		if len(availableProjectNames) > 0 {
			sort.Strings(availableProjectNames)
			availableProjectsDetail = fmt.Sprintf("Available projects: %s.", strings.Join(availableProjectNames, ", "))
		}

		return nil, &SummaryWithDetailError{
			Summary: "Project not found",
			Detail:  fmt.Sprintf("No project with the name '%s' was found. Set project_name to a valid project name. %s", projectName, availableProjectsDetail),
		}
	}

	if len(sameNameProjectIDs) > 1 {
		return nil, &SummaryWithDetailError{
			Summary: "Multiple projects found",
			Detail:  fmt.Sprintf("Multiple projects named '%s' were found. Please rename projects to use a unique project name for %s assignment.", projectName, kind),
		}
	}

	projectID := sameNameProjectIDs[0]

	return &projectID, nil
}
