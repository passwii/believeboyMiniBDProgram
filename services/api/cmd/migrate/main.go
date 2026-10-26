package main

import (
	"fmt"
	"os"

	"github.com/believeboy/mini-bd-program/services/api/internal/database"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if err := database.Migrate(url, command); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
