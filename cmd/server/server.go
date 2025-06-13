package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/chitransh-rockwallet/1sat-indexer/config"
	"github.com/chitransh-rockwallet/1sat-indexer/idx"
	"github.com/chitransh-rockwallet/1sat-indexer/server"
	"github.com/joho/godotenv"
)

var PORT int
var CONCURRENCY uint
var VERBOSE int

func init() {
	wd, _ := os.Getwd()
	log.Println("CWD:", wd)
	godotenv.Load(fmt.Sprintf(`%s/../../.env`, wd))

	PORT, _ = strconv.Atoi(os.Getenv("PORT"))
	flag.IntVar(&PORT, "p", PORT, "Port to listen on")
	flag.UintVar(&CONCURRENCY, "c", 1, "Concurrency")
	flag.IntVar(&VERBOSE, "v", 0, "Verbose")
	flag.Parse()
}

func main() {
	app := server.Initialize(&idx.IngestCtx{
		Tag:         idx.IngestTag,
		Indexers:    config.Indexers,
		Concurrency: CONCURRENCY,
		Network:     config.Network,
		Once:        true,
		Store:       config.Store,
		// Verbose:     VERBOSE > 0,
		Verbose: true,
	}, config.Broadcaster)
	log.Println("Listening on", PORT)
	app.Listen(fmt.Sprintf(":%d", PORT))
}
