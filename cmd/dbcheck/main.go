package main

import (
	"log"

	"github.com/mgierada/calculon/internal/db"
)

func main() {
	if err := db.PrintVersion(); err != nil {
		log.Fatal(err)
	}
}
