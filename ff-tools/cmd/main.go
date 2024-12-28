package main

import (
	"log"

	"github.com/Despire/ff-tools/cmd/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		log.Fatal(err)
	}
}
