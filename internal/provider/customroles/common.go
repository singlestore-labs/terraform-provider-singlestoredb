package customroles

type ResourceType string

const (
	ResourceTypeOrganization   ResourceType = "Organization"
	ResourceTypeWorkspaceGroup ResourceType = "Cluster"
	ResourceTypeTeam           ResourceType = "Team"
	ResourceTypeSecret         ResourceType = "Secret"
)

var ResourceTypeList = []ResourceType{
	ResourceTypeOrganization,
	ResourceTypeWorkspaceGroup,
	ResourceTypeTeam,
	ResourceTypeSecret,
}
