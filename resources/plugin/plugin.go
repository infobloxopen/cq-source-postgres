package plugin

import (
	_ "embed"

	"github.com/cloudquery/plugin-sdk/v4/plugin"
)

var (
	Name    = "postgresql"
	Kind    = "source"
	Team    = "infobloxopen"
	Version = "development"
)

//go:embed schema.json
var jsonSchema string

func Plugin() *plugin.Plugin {
	return plugin.NewPlugin(Name, Version, Configure,
		plugin.WithKind(Kind),
		plugin.WithTeam(Team),
		plugin.WithJSONSchema(jsonSchema),
	)
}
