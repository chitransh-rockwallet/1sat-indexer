package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/chitransh-rockwallet/1sat-indexer/config"
	"github.com/chitransh-rockwallet/1sat-indexer/evt"
	"github.com/chitransh-rockwallet/1sat-indexer/idx"
	redisstore "github.com/chitransh-rockwallet/1sat-indexer/idx/redis-store"
	"github.com/chitransh-rockwallet/1sat-indexer/mod/onesat"
	"github.com/chitransh-rockwallet/1sat-indexer/sub"
)

var TAG = "bsv21"
var PAGE_SIZE = uint32(1000)
var CONCURRENCY uint
var TOPIC string
var VERBOSE uint

var ctx = context.Background()
var store *redisstore.RedisStore

func init() {
	var err error
	if store, err = redisstore.NewRedisStore(os.Getenv("REDISTXO")); err != nil {
		panic(err)
	}
}

var ingest = &idx.IngestCtx{
	Tag: TAG,
	Indexers: []idx.Indexer{
		&onesat.InscriptionIndexer{},
		&onesat.Bsv21Indexer{},
		&onesat.OrdLockIndexer{},
	},
	Network:     config.Network,
	Concurrency: 1,
	PageSize:    PAGE_SIZE,
}

func main() {
	flag.StringVar(&TOPIC, "t", "", "Junglebus SubscriptionID")
	flag.UintVar(&CONCURRENCY, "c", 1, "Concurrency")
	flag.UintVar(&VERBOSE, "v", 0, "Verbose")
	flag.Parse()

	limiter := make(chan struct{}, CONCURRENCY)

	if TOPIC != "" {
		go subscribe()
	}
	go categorize()
	if tokenIds, err := store.SearchOutpoints(ctx, &idx.SearchCfg{
		Keys: []string{evt.EventKey(onesat.BSV21_TAG, &evt.Event{
			Id:    "issue",
			Value: "",
		})},
		Limit: 0,
	}); err != nil {
		log.Panic(err)
	} else {
		for _, tokenId := range tokenIds {
			limiter <- struct{}{}
			go func(tokenId string) {
				defer func() { <-limiter }()
				if err := processToken(tokenId); err != nil {
					log.Println("Error processing token", tokenId, err)
				}
			}(tokenId)
		}
		if len(tokenIds) == 0 {
			// log.Println("No results")
			time.Sleep(time.Second)
		}
	}
}

func processToken(tokenId string) (err error) {
	if tokenTxo, err := store.LoadTxo(ctx, tokenId, []string{onesat.BSV21_TAG}, false, false); err != nil {
		log.Println("Error loading token", tokenId, err)
		return err
	} else if tokenTxo == nil {
		log.Println("Missing token", tokenId, err)
		return err
	} else if bsv21Data, ok := tokenTxo.Data[onesat.BSV21_TAG]; !ok {
		log.Println("Missing BSV21 data", tokenId)
		return err
	} else {
		bsv21 := bsv21Data.Data.(*onesat.Bsv21)
		store.LogOnce(ctx, idx.OwnerSyncKey, bsv21.FundAddress, 0)
		// pendingKey := evt.EventKey(onesat.BSV21_TAG, &evt.Event{
		// 	Id:    onesat.PendingEvent,
		// 	Value: bsv21.Id,
		// })
		// if pendingCount, err := idx.TxoDB.ZCard(ctx, pendingKey).Result(); err != nil {
		// 	log.Println("Error counting pending", bsv21.FundAddress, err)
		// 	return err
		// } else if pendingCount == 0 {
		// 	return nil
		// }

		if balance, err := store.Balance(ctx, idx.OwnerKey(bsv21.FundAddress)); err != nil {
			log.Println("Error getting balance", tokenId, bsv21.FundAddress, err)
			return err
		} else if validCount, err := store.CountMembers(ctx, evt.EventKey(onesat.BSV21_TAG, &evt.Event{
			Id:    onesat.ValidEvent,
			Value: bsv21.Id,
		})); err != nil {
			log.Println("Error counting valid", tokenId, bsv21.FundAddress, err)
			return err
		} else if invalidCount, err := store.CountMembers(ctx, evt.EventKey(onesat.BSV21_TAG, &evt.Event{
			Id:    onesat.InvalidEvent,
			Value: bsv21.Id,
		})); err != nil {
			log.Println("Error counting invalid", tokenId, bsv21.FundAddress, err)
			return err
		} else {
			validationCount := int64(validCount + invalidCount)
			for balance = balance - (validationCount * onesat.BSV21_INDEX_FEE); balance > 0; {
				if items, err := store.Search(ctx, &idx.SearchCfg{
					Keys:  []string{idx.QueueKey(tokenId)},
					Limit: 1,
				}); err != nil {
					log.Println("Error getting queue", tokenId, err)
					return err
				} else if len(items) == 0 {
					break
				} else {
					// txid := items[0]
					// if idxCtx, err := ingest.ParseTxid(ctx, txid, idx.AncestorConfig{
					// 	Load:  true,
					// 	Parse: true,
					// }); err != nil {
					// 	log.Println("Error parsing txid", txid, err)
					// 	return err
					// } else {
					// for _, txo := range idxCtx.Txos {
					// 	if idxData, ok := txo.Data[onesat.BSV21_TAG]; ok {
					// 		if bsv21, ok := idxData.Data.(*onesat.Bsv21); ok {
					// 			if bsv21.Id != tokenId || bsv21.Status == onesat.Pending {
					// 				continue
					// 			}
					// 			idx.TxoDB.ZRem(ctx, idx.QueueKey(tokenId), txo.Outpoint.String())
					// 		}
					// 	}
					// }
					// if err := txo.Save(ctx, idxCtx.Height, idxCtx.Idx); err != nil {
					// 	log.Println("Error saving txo", txid, err)
					// 	return err
					// }
					// }
				}
			}
			// if   {
			// 	log.Println("Insufficient balance", tokenId, bsv21.FundAddress, balance, validationCost)
			// 	return nil
			// }

			// limit := uint32((balance - uint64(validationCost)) / onesat.BSV21_INDEX_FEE)
			// if limit == 0 {
			// 	log.Println("Insufficient balance", tokenId, bsv21.FundAddress, balance, validationCost)
			// 	return nil
			// }
			// tokenIngest := *ingest
			// tokenIngest.Limit = limit
			// onIngest := func(ctx context.Context, idxCtx *idx.IndexContext) error {
			// 	for _, txo := range idxCtx.Txos {
			// 		if idxData, ok := txo.Data[onesat.BSV21_TAG]; ok {
			// 			if bsv21, ok := idxData.Data.(*onesat.Bsv21); ok {
			// 				if bsv21.Id != tokenId || bsv21.Status == onesat.Pending {
			// 					continue
			// 				}
			// 				idx.TxoDB.ZRem(ctx, pendingKey, txo.Outpoint.String())
			// 			}
			// 		}
			// 	}
			// 	return nil
			// }
			// tokenIngest.OnIngest = &onIngest
			// if err := tokenIngest.Exec(ctx); err != nil {
			// 	log.Println("Error ingesting token", tokenId, err)
			// 	return err
			// }
		}
	}
	return nil
}

var queueKey = idx.QueueKey(TAG)

func subscribe() {
	if err := (&sub.Sub{
		Tag:          TAG,
		Queue:        TAG,
		Topic:        TOPIC,
		FromBlock:    801000,
		IndexBlocks:  true,
		IndexMempool: true,
		Verbose:      VERBOSE > 0,
	}).Exec(ctx); err != nil {
		log.Panic(err)
	}
}

func categorize() {
	if txids, err := store.SearchOutpoints(ctx, &idx.SearchCfg{
		Keys: []string{queueKey},
	}); err != nil {
		log.Panic(err)
	} else {
		limiter := make(chan struct{}, CONCURRENCY)
		// errors := make(chan error)
		for _, txid := range txids {
			go func(txid string) {
				defer func() {
					<-limiter
					// done <- txid
				}()
				if idxCtx, err := ingest.ParseTxid(ctx, txid, idx.AncestorConfig{
					Load:  true,
					Parse: true,
				}); err != nil {
					panic(err)
				} else {
					for _, txo := range idxCtx.Txos {
						if bsv21Data, ok := txo.Data[onesat.BSV21_TAG]; ok {
							bsv21 := bsv21Data.Data.(*onesat.Bsv21)
							if bsv21.Op == "deploy+mint" {
								if err = store.SaveTxo(ctx, txo, idxCtx.Height, idxCtx.Idx); err != nil {
									panic(err)
								}
							} else {
								store.LogOnce(ctx, idx.QueueKey(bsv21.Id), txo.Outpoint.TxidHex(), 0)
							}
						}
					}

					if err = store.Delog(ctx, queueKey, txid); err != nil {
						log.Panic(err)
					}
				}
			}(txid)
		}
		if len(txids) == 0 {
			// log.Println("No transactions to ingest")
			time.Sleep(time.Second)
		}
	}
}
