// Command kandev-plugin-forgejo is the backend half of this Kandev plugin.
// Kandev spawns it as a gRPC subprocess; pluginsdk.Serve owns the transport.
package main

import (
	"kandev-plugin-forgejo/internal/plugin"

	"github.com/kandev/kandev/pkg/pluginsdk"
)

func main() {
	pluginsdk.Serve(plugin.NewRuntime())
}
