package main

import (
	"log"

	"github.com/catalog-release-bot/catalog-release-bot/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
