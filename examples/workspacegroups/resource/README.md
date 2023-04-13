# Workspace Groups Resource

`workspacegroups/resource` shows operations on top of the worspace group resource.

~~~ shell
terraform apply # Create.

# Manually update name/expires_at/admin_password in `main.tf` to present a change.

terraform apply # Read & Update.

terraform destroy # Delete.
~~~

**Note: This Terraform provider is currently unpublished on the Terraform Registry and can only be executed in your local environment.**

**Note: `terraform init` does not work with `dev_overrides` for local development.**
