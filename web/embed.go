package web

import "embed"

// DistFS contains the built React SPA assets.
//
//go:embed dist/*
//go:embed dist/assets/*
var DistFS embed.FS
