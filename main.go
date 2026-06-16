package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider"
)

var version = "dev" // Version is populated by goreleaser with ldflags.

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	ctx := context.Background()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/singlestore-labs/singlestoredb",
		Debug:   debug,
	}

	if err := providerserver.Serve(ctx, provider.New(version), opts); err != nil {
		log.Fatal(err)
	}
}
