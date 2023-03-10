# regions

`workspacegroups/resource` shows operations on top of the worspace group resource.

~~~ shell
terraform apply # Create.

# Manually update name/expires_at/admin_password in `main.tf` to present a change.

terraform apply # Read & Update.

terraform destroy # Delete.
~~~

**Note: `terraform init` does not work with `dev_overrides` for local development.**
