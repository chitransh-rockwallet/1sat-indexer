package main

import (
	"context"
	"flag"
	"log"
	"sync"
	"time"

	"github.com/chitransh-rockwallet/1sat-indexer/config"
	"github.com/chitransh-rockwallet/1sat-indexer/evt"
	"github.com/chitransh-rockwallet/1sat-indexer/idx"
	"github.com/chitransh-rockwallet/1sat-indexer/mod/onesat"
)

var MAX_DEPTH uint
var CONCURRENCY uint
var ctx = context.Background()

// var originIndexer = &onesat.OriginIndexer{}
// var inscIndexer = &onesat.InscriptionIndexer{}

var ingest *idx.IngestCtx

var eventKey = evt.EventKey("origin", &evt.Event{
	Id:    "outpoint",
	Value: "",
})

var queue = make(chan string, 10000000)
var processed map[string]struct{}
var wg sync.WaitGroup

func init() {
	ingest = &idx.IngestCtx{
		Tag:      "origin",
		Indexers: config.Indexers,
		Network:  config.Network,
		Store:    config.Store,
	}
}

func main() {
	flag.UintVar(&CONCURRENCY, "c", 1, "Concurrency")
	flag.Parse()
	go func() {
		failed := make(chan string, 100)
		limiter := make(chan struct{}, CONCURRENCY)
		for {
			select {
			case txid := <-queue:
				if _, ok := processed[txid]; !ok {
					processed[txid] = struct{}{}
					limiter <- struct{}{}
					go func(txid string) {
						defer func() {
							<-limiter
							wg.Done()
						}()
						if err := ResolveOrigins(txid); err != nil {
							log.Println()
							failed <- txid
						}
					}(txid)
				} else {
					wg.Done()
				}
			case txid := <-failed:
				delete(processed, txid)
			}
		}
	}()

	for {
		searchCfg := &idx.SearchCfg{
			Keys:  []string{eventKey},
			Limit: 1000000,
			// Reverse: true,
		}
		if outpoints, err := config.Store.SearchOutpoints(ctx, searchCfg); err != nil {
			log.Panic(err)
		} else {
			processed = make(map[string]struct{}, 10000)
			txids := make(map[string]struct{}, len(outpoints))
			for _, outpoint := range outpoints {
				txids[outpoint[:64]] = struct{}{}
			}
			log.Println("Calculating origins for", len(txids), "txns")
			for txid := range txids {
				wg.Add(1)
				queue <- txid
			}
			wg.Wait()
			if len(outpoints) == 0 {
				time.Sleep(time.Second)
				log.Println("No results")
			}
		}
	}
}

func ResolveOrigins(txid string) (err error) {
	if idxCtx, err := ingest.IngestTxid(ctx, txid, idx.AncestorConfig{
		Load:  true,
		Parse: true,
		Save:  true,
	}); err != nil {
		log.Println("ingest-err", txid, err)
		// if err == jb.ErrMissingTxn {
		// 	log.Println("resolve missing-txn", txid)
		// 	return nil
		// }
		// log.Panic(err)
		return err
	} else if idxCtx == nil {
		return nil
	} else {
		for _, spend := range idxCtx.Spends {
			if spend.Data[onesat.ORIGIN_TAG] != nil {
				origin := spend.Data[onesat.ORIGIN_TAG].Data.(*onesat.Origin)
				if origin.Outpoint == nil {
					log.Println("Queuing parent:", spend.Outpoint.String())
					wg.Add(1)
					queue <- spend.Outpoint.TxidHex()
				}
			}
		}
		resolved := make([]string, 0, len(idxCtx.Txos))
		for _, txo := range idxCtx.Txos {
			if txo.Data[onesat.ORIGIN_TAG] != nil {
				origin := txo.Data[onesat.ORIGIN_TAG].Data.(*onesat.Origin)
				if origin.Outpoint != nil {
					op := txo.Outpoint.String()
					resolved = append(resolved, op)
				}
			}
		}
		if len(resolved) > 0 {
			log.Println("Resolved", resolved)
			if err := config.Store.Delog(ctx, eventKey, resolved...); err != nil {
				log.Panic(err)
			}
		}
	}
	return nil
}

// func ResolveOrigin(outpoint string, depth uint32) (origin *onesat.Origin, err error) {
// 	var txo *idx.Txo
// 	if txo, err = store.LoadTxo(ctx, outpoint, []string{onesat.ORIGIN_TAG}); err != nil {
// 		log.Panic(err)
// 		return nil, err
// 	} else if txo == nil {
// 		log.Panic("Missing txo:", outpoint)
// 	}
// 	origin = txo.Data[onesat.ORIGIN_TAG].Data.(*onesat.Origin)
// 	if origin.Outpoint == nil {

// 	}
// 	return origin, nil
// }
