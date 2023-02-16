package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider"
)

func main() {
	ctx := context.Background()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/singlestoredb/singlestore",
	}

	if err := providerserver.Serve(ctx, provider.New(), opts); err != nil {
		log.Fatal(err)
	}
}
