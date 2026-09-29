package main

import (
	"fmt"
	"log"
	"warehouse-api/internal/config"
	"warehouse-api/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	r := server.NewRouter(cfg)
	if err := r.Run(fmt.Sprintf(":%d", cfg.Server.Port)); err != nil {
		log.Fatal(err)
	}
}
