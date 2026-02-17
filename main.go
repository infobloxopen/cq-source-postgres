package main

import (
	"context"
	"log"

	"github.com/cloudquery/plugin-sdk/v4/serve"
	internalPlugin "github.com/infobloxopen/cq-source-postgres/resources/plugin"
)

func main() {
	p := internalPlugin.Plugin()
	if err := serve.Plugin(p).Serve(context.Background()); err != nil {
		log.Fatalf("failed to serve plugin: %v", err)
	}
}
