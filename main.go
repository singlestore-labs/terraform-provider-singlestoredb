package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider"
)

var version = "dev" // Version is populated by goreleaser with ldflags.

func main() {
	ctx := context.Background()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/singlestore-labs/singlestoredb",
	}

	if err := providerserver.Serve(ctx, provider.New(version), opts); err != nil {
		log.Fatal(err)
	}
}
