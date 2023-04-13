# Workspaces Resource

`workspaces/resource` shows operations on top of the worspace resource.

~~~ shell
terraform apply # Create.

# Connect to the workspace and execute 'select 1'.

export endpoint=$(terraform output -raw example_endpoint)

mysql -u admin -h $endpoint -P 3306 --default-auth=mysql_native_password --password='fooBAR12$' -e 'select 1'

# Manually update size to 0 in `main.tf` to suspend.

terraform apply # Suspend.

# Manually update size to 0.25 in `main.tf` to resume.

terraform apply # Resume.

# Manually update size to 0.5 in `main.tf` to scale.

terraform apply # Scale.

terraform destroy # Delete.
~~~

**Note: This Terraform provider is currently unpublished on the Terraform Registry and can only be executed in your local environment.**

**Note: `terraform init` does not work with `dev_overrides` for local development.**
