package db

import "embed"

// Migrations contains the versioned SQL migration files.
//
//go:embed migrations/*.sql
var Migrations embed.FS
