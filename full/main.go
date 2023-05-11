package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/GorillaPool/go-junglebus"
	jbModels "github.com/GorillaPool/go-junglebus/models"
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/joho/godotenv"
	"github.com/libsv/go-bt/v2"
	"github.com/redis/go-redis/v9"
	"github.com/shruggr/1sat-indexer/indexer"
	"github.com/shruggr/1sat-indexer/lib"
)

const INDEXER = "full"

var THREADS uint64 = 16

var db *sql.DB
var junglebusClient *junglebus.Client
var msgQueue = make(chan *Msg, 1000000)

// fdb.MustAPIVersion(620)
var foundation fdb.Database

// var settled = make(chan uint32, 1000)
var fromBlock uint32
var sub *junglebus.Subscription

type Msg struct {
	Id          string
	Height      uint32
	Hash        string
	Status      uint32
	Idx         uint64
	Transaction []byte
}

// var ctx = context.Background()
var rdb *redis.Client

func init() {
	godotenv.Load("../.env")

	// cluster := gocql.NewCluster("server")
	// cluster.Keyspace = "spends"

	var err error
	// rabbit, err = amqp.Dial(os.Getenv("RABBITMQ"))
	// if err != nil {
	// 	log.Panic(err)
	// }

	db, err = sql.Open("postgres", os.Getenv("POSTGRES"))
	if err != nil {
		log.Panic(err)
	}
	db.SetConnMaxIdleTime(time.Millisecond * 100)
	db.SetMaxOpenConns(400)
	db.SetMaxIdleConns(25)

	fdb.MustAPIVersion(620)
	foundation = fdb.MustOpenDefault()

	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	err = lib.Initialize(db, rdb, foundation)
	if err != nil {
		log.Panic(err)
	}

	if os.Getenv("THREADS") != "" {
		THREADS, err = strconv.ParseUint(os.Getenv("THREADS"), 10, 64)
		if err != nil {
			log.Panic(err)
		}
	}
}

func main() {
	var err error
	junglebusClient, err = junglebus.New(
		junglebus.WithHTTP("https://junglebus.gorillapool.io"),
	)
	if err != nil {
		log.Panicln(err.Error())
	}
	row := db.QueryRow(`SELECT height
		FROM progress
		WHERE indexer=$1`,
		INDEXER,
	)
	row.Scan(&fromBlock)
	// if fromBlock < lib.TRIGGER {
	// 	fromBlock = lib.TRIGGER
	// }

	go processQueue()
	subscribe()
	defer func() {
		if r := recover(); r != nil {
			sub.Unsubscribe()
			fmt.Println("Recovered in f", r)
			fmt.Println("Unsubscribing and exiting...")
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Printf("Caught signal")
		fmt.Println("Unsubscribing and exiting...")
		sub.Unsubscribe()
		os.Exit(0)
	}()

	var wg2 sync.WaitGroup
	wg2.Add(1)
	wg2.Wait()
}

func subscribe() {
	var err error
	log.Println("Subscribing to Junglebus from block", fromBlock)
	sub, err = junglebusClient.Subscribe(
		context.Background(),
		os.Getenv("FULL"),
		uint64(fromBlock),
		junglebus.EventHandler{
			OnTransaction: func(txResp *jbModels.TransactionResponse) {
				// log.Printf("[TX]: %d - %d: %s\n", txResp.BlockHeight, txResp.BlockIndex, txResp.Id)
				msgQueue <- &Msg{
					Id:          txResp.Id,
					Height:      txResp.BlockHeight,
					Idx:         txResp.BlockIndex,
					Transaction: txResp.Transaction,
				}
			},
			OnStatus: func(status *jbModels.ControlResponse) {
				// log.Printf("[STATUS]: %v\n", status)
				msgQueue <- &Msg{
					Height: status.Block,
					Status: status.StatusCode,
				}
			},
			OnError: func(err error) {
				log.Printf("[ERROR]: %v", err)
			},
		},
	)
	if err != nil {
		log.Panic(err)
	}
}

func processQueue() {
	var settledHeight uint32
	// go indexer.ProcessInscriptionIds(settled)
	go indexer.ProcessTxns(uint(THREADS))
	for {
		msg := <-msgQueue

		switch msg.Status {
		case 0:
			tx, err := bt.NewTxFromBytes(msg.Transaction)
			if err != nil {
				log.Panicf("OnTransaction Parse Error: %s %d %+v\n", msg.Id, len(msg.Transaction), err)
			}
			lib.TxCache.Add(msg.Id, tx)

			txn := &indexer.TxnStatus{
				ID:       msg.Id,
				Tx:       tx,
				Height:   msg.Height,
				Idx:      uint64(msg.Idx),
				Parents:  map[string]*indexer.TxnStatus{},
				Children: map[string]*indexer.TxnStatus{},
			}

			// _, err = lib.SetTxn.Exec(msg.Id, msg.Hash, txn.Height, txn.Idx)
			// if err != nil {
			// 	panic(err)
			// }

			indexer.M.Lock()
			_, ok := indexer.Txns[msg.Id]
			indexer.M.Unlock()
			if ok {
				continue
			}
			for _, input := range tx.Inputs {
				inTxid := input.PreviousTxIDStr()
				indexer.M.Lock()
				if parent, ok := indexer.Txns[inTxid]; ok {
					parent.Children[msg.Id] = txn
					txn.Parents[parent.ID] = parent
				}
				indexer.M.Unlock()
			}
			indexer.M.Lock()
			indexer.Txns[msg.Id] = txn
			indexer.M.Unlock()
			if len(txn.Parents) == 0 {
				indexer.Wg.Add(1)
				indexer.InQueue++
				indexer.TxnQueue <- txn
			}
		// On Connected, if already connected, unsubscribe and cool down

		case 200:
			indexer.Wg.Wait()
			if msg.Height > 5 {
				settledHeight = msg.Height - 6
			}

			if _, err := db.Exec(`INSERT INTO progress(indexer, height)
				VALUES($1, $2)
				ON CONFLICT(indexer) DO UPDATE
					SET height=$2`,
				INDEXER,
				settledHeight,
			); err != nil {
				log.Panic(err)
			}
			fromBlock = msg.Height + 1
			fmt.Printf("Completed: %d\n", msg.Height)
			// settled <- settledHeight

		default:
			log.Printf("Status: %d\n", msg.Status)
		}
	}
}
