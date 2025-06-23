package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/chitransh-rockwallet/1sat-indexer/v5/config"
	"github.com/chitransh-rockwallet/1sat-indexer/v5/idx"
	"github.com/chitransh-rockwallet/1sat-indexer/v5/jb"
)

var ctx = context.Background()
var CONCURRENCY int
var TAG string
var store idx.TxoStore

func init() {
	flag.StringVar(&TAG, "tag", idx.IngestTag, "Ingest tag")
	flag.IntVar(&CONCURRENCY, "c", 1, "Concurrency")
	flag.Parse()
	store = config.Store
}

var mempoolScore = idx.HeightScore(50000000, 0)

func main() {
	for {
		if results, err := store.Search(ctx, &idx.SearchCfg{
			Keys: []string{idx.OwnerSyncKey},
		}); err != nil {
			log.Panic(err)
		} else if len(results) == 0 {
			time.Sleep(time.Minute)
			continue
		} else {
			for _, result := range results {
				lastHeight := int(result.Score)
				log.Println("Owner", result.Member, "lastHeight", lastHeight)
				start := time.Now()
				if addTxns, err := jb.FetchOwnerTxns(result.Member, lastHeight); err == jb.ErrBadRequest {
					log.Println("ErrBadRequest", result.Member)
					continue
				} else if err != nil {
					log.Panic(err)
				} else {
					log.Println("Fetched", len(addTxns), "txns in", time.Since(start))
					for _, addTxn := range addTxns {
						if addTxn.Height > uint32(lastHeight) {
							lastHeight = int(addTxn.Height)
						}
						if score, err := store.LogScore(ctx, idx.LogKey(TAG), addTxn.Txid); err != nil && err != redis.Nil {
							log.Panic(err)
						} else if score > 0 && score <= mempoolScore {
							log.Println("Skipping", addTxn.Txid, score)
							continue
						}
						score := idx.HeightScore(addTxn.Height, addTxn.Idx)
						if err := store.Log(ctx, idx.QueueKey(TAG), addTxn.Txid, score); err != nil {
							log.Panic(err)
						}
						log.Println("Queuing", addTxn.Txid, score)
					}

					if err := store.Log(ctx, idx.OwnerSyncKey, result.Member, float64(lastHeight)); err != nil {
						log.Panic(err)
					}
				}
			}
		}
		time.Sleep(time.Minute)
	}
}
