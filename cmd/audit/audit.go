package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/chitransh-rockwallet/1sat-indxer/audit"
	"github.com/chitransh-rockwallet/1sat-indxer/config"
	"github.com/chitransh-rockwallet/1sat-indxer/idx"
	"github.com/joho/godotenv"
)

func init() {
	wd, _ := os.Getwd()
	log.Println("CWD:", wd)
	godotenv.Load(fmt.Sprintf(`%s/../../.env`, wd))
}

func main() {
	audit.StartTxAudit(context.Background(), &idx.IngestCtx{
		Indexers: config.Indexers,
		Network:  config.Network,
		Store:    config.Store,
	}, config.Broadcaster, true)
}
