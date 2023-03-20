# Workspace Groups Import

`workspacegroups/import` shows importing a workspace group that already exists.

~~~ shell
# Replace with the ID of the group that exists in portal https://portal.singlestore.com/.
# Consider listing workspace group with the datasource workspace_groups to fetch the ID.
export WORKSPACE_GROUP_ID=59a0f404-4c23-4541-8fb6-c55f5a23e290

terraform import singlestore_workspace_group.example $WORKSPACE_GROUP_ID

# The command should succeed. Inspect the result, especially the `region_id` field.
terraform state show singlestore_workspace_group.example

# Manually update `region_id` in `main.tf` to equal to the actual region ID value.

terraform apply # This should propose some updates to the imported resource.
~~~

**Note: `terraform init` does not work with `dev_overrides` for local development.**
