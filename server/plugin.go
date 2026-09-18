package main

import "kandev-plugin-forgejo/internal/plugin"

// forgejoPlugin is the value Serve runs; the implementation lives in
// internal/plugin so it can be unit tested without the main package.
type forgejoPlugin = plugin.Runtime
