package main

import (
	"log"

	"github.com/mgierada/calculon/internal/config"
	"github.com/mgierada/calculon/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if err := db.PrintVersion(cfg.DBPath); err != nil {
		log.Fatal(err)
	}
}
