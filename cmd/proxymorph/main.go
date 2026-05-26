package main

import (
	"fmt"
	"log"

	"github.com/liulei/proxymorph/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ProxyMorph listening on %s\n", cfg.Addr)
}
