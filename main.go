package main

import (
	"embed"
	"io/fs"
	"log"

	"zarkham/cli/cmd"
)

//go:embed gui-assets/*
var guiAssets embed.FS

func main() {
	// Subtree to get the content inside gui-assets
	guiFS, err := fs.Sub(guiAssets, "gui-assets")
	if err != nil {
		log.Fatalf("Failed to embed GUI: %v", err)
	}
	cmd.Execute(guiFS)
}