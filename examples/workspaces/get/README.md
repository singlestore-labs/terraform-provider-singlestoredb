# regions

`workspaces/get` shows workspace lookup by ID.

~~~ shell
# Replace `id` in `main.tf` with the ID of the workspace that exists.
# To fetch the ID, consider leveraging the `workspaces` data source.
terraform apply -auto-approve
~~~

**Note: `terraform init` does not work with `dev_overrides` for local development.**
