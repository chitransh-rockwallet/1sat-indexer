package lib

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/GorillaPool/go-junglebus"
	"github.com/GorillaPool/go-junglebus/models"
	lru "github.com/hashicorp/golang-lru/v2"
	_ "github.com/lib/pq"
	"github.com/libsv/go-bt/v2"
	"github.com/tikv/client-go/v2/txnkv"
)

var TRIGGER = uint32(783968)

var Tikv *txnkv.Client
var txCache *lru.ARCCache[string, *models.Transaction]
var Db *sql.DB
var JBClient *junglebus.Client

var GetInsNumber *sql.Stmt
var GetTxo *sql.Stmt
var GetTxos *sql.Stmt
var GetInput *sql.Stmt
var GetInsciption *sql.Stmt
var GetInsciptions *sql.Stmt
var GetInsciptionByID *sql.Stmt
var GetInsciptionsByTxID *sql.Stmt
var GetMaxInscriptionId *sql.Stmt
var GetUnnumberedIns *sql.Stmt

// var InsSpend *sql.Stmt
var InsTxo *sql.Stmt
var InsInscription *sql.Stmt
var SetSpend *sql.Stmt
var SetTxoOrigin *sql.Stmt
var SetInscriptionOrigin *sql.Stmt
var SetInscriptionId *sql.Stmt
var GetUtxos *sql.Stmt

func Initialize(db *sql.DB) (err error) {
	Db := db
	jb := os.Getenv("JUNGLEBUS")
	if jb == "" {
		jb = "https://junglebus.gorillapool.io"
	}
	JBClient, err = junglebus.New(
		junglebus.WithHTTP(jb),
	)
	if err != nil {
		return
	}

	Tikv, err = txnkv.NewClient([]string{os.Getenv("TIKV")})
	if err != nil {
		log.Panicln(err.Error())
	}

	GetInsNumber, err = Db.Prepare(`
		SELECT COUNT(i.txid) 
		FROM inscriptions i
		JOIN inscriptions l ON i.height < l.height OR (i.height = l.height AND i.idx < l.idx)
		WHERE l.txid=$1 AND l.vout=$2
	`)
	if err != nil {
		return
	}

	GetTxo, err = Db.Prepare(`SELECT txid, vout, satoshis, acc_sats, lock, COALESCE(spend, '\x'::BYTEA), COALESCE(origin, '\x'::BYTEA)
		FROM txos
		WHERE txid=$1 AND vout=$2 AND acc_sats IS NOT NULL
	`)
	if err != nil {
		log.Fatal(err)
	}

	GetTxos, err = Db.Prepare(`SELECT txid, vout, satoshis, acc_sats, lock, COALESCE(spend, '\x'::BYTEA), COALESCE(origin, '\x'::BYTEA)
		FROM txos
		WHERE txid=$1 AND satoshis=1 AND acc_sats IS NOT NULL
	`)
	if err != nil {
		log.Fatal(err)
	}

	GetUtxos, err = Db.Prepare(`SELECT txid, vout, satoshis, acc_sats, lock, COALESCE(spend, '\x'::BYTEA), COALESCE(origin, '\x'::BYTEA)
		FROM txos
		WHERE lock=$1 AND spend IS NULL
	`)
	if err != nil {
		log.Fatal(err)
	}

	GetInput, err = Db.Prepare(`SELECT txid, vout, satoshis, acc_sats, lock, COALESCE(spend, '\x'::BYTEA), COALESCE(origin, '\x'::BYTEA)
		FROM txos
		WHERE spend=$1 AND acc_sats>=$2 AND satoshis=1
		ORDER BY acc_sats ASC
		LIMIT 1
	`)
	if err != nil {
		log.Fatal(err)
	}

	GetInsciption, err = Db.Prepare(`SELECT txid, vout, height, idx, filehash, filesize, filetype, COALESCE(id, 0), COALESCE(origin, '\x'::BYTEA), lock
		FROM inscriptions
		WHERE origin=$1
		ORDER BY height DESC, idx DESC
		LIMIT 1`,
	)
	if err != nil {
		log.Fatal(err)
	}

	GetInsciptions, err = Db.Prepare(`SELECT txid, vout, height, idx, filehash, filesize, filetype, COALESCE(id, 0), COALESCE(origin, '\x'::BYTEA), lock
		FROM inscriptions
		WHERE origin=$1
		ORDER BY height DESC, idx DESC`,
	)
	if err != nil {
		log.Fatal(err)
	}

	GetInsciptionByID, err = Db.Prepare(`SELECT txid, vout, height, idx, filehash, filesize, filetype, COALESCE(id, 0), COALESCE(origin, '\x'::BYTEA), lock
		FROM inscriptions
		WHERE id=$1`,
	)

	GetInsciptionsByTxID, err = Db.Prepare(`SELECT txid, vout, height, idx, filehash, filesize, filetype, COALESCE(id, 0), COALESCE(origin, '\x'::BYTEA), lock
		FROM inscriptions
		WHERE txid=$1
		ORDER BY vout DESC`,
	)
	if err != nil {
		log.Fatal(err)
	}

	GetMaxInscriptionId, err = Db.Prepare(`SELECT MAX(id) FROM inscriptions`)
	if err != nil {
		log.Fatal(err)
	}

	GetUnnumberedIns, err = Db.Prepare(`
		SELECT txid, vout 
		FROM inscriptions
		WHERE id IS NULL AND height <= $1
		ORDER BY height, idx, vout`,
	)
	if err != nil {
		log.Fatal(err)
	}

	InsTxo, err = Db.Prepare(`INSERT INTO txos(txid, vout, satoshis, acc_sats, lock, origin, height, idx)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT(txid, vout) DO UPDATE SET 
			lock=EXCLUDED.lock, 
			satoshis=EXCLUDED.satoshis,
			height=EXCLUDED.height,
			idx=EXCLUDED.idx
	`)
	if err != nil {
		log.Fatal(err)
	}

	SetTxoOrigin, err = Db.Prepare(`UPDATE txos
		SET origin=$3
		WHERE txid=$1 AND vout=$2
	`)
	if err != nil {
		log.Fatal(err)
	}

	SetInscriptionOrigin, err = Db.Prepare(`UPDATE inscriptions
		SET origin=$3
		WHERE txid=$1 AND vout=$2
	`)
	if err != nil {
		log.Fatal(err)
	}

	SetInscriptionId, err = Db.Prepare(`UPDATE inscriptions
		SET id=$3
		WHERE txid=$1 AND vout=$2
	`)
	if err != nil {
		log.Fatal(err)
	}

	SetSpend, err = Db.Prepare(`UPDATE txos
		SET spend=$3, vin=$4
		WHERE txid=$1 AND vout=$2
	`)
	if err != nil {
		log.Fatal(err)
	}

	// InsSpend, err = Db.Prepare(`INSERT INTO txos(txid, vout, spend, vin)
	// 	VALUES($1, $2, $3, $4)
	// 	ON CONFLICT(txid, vout) DO UPDATE
	// 		SET spend=EXCLUDED.spend, vin=EXCLUDED.vin
	// `)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	InsInscription, err = Db.Prepare(`
		INSERT INTO inscriptions(txid, vout, height, idx, filehash, filesize, filetype, origin, lock)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT(txid, vout) DO UPDATE
			SET height=EXCLUDED.height, idx=EXCLUDED.idx
	`)
	if err != nil {
		log.Panic(err)
	}

	txCache, err = lru.NewARC[string, *models.Transaction](2 ^ 30)
	return
}

func LoadTx(txid []byte) (tx *bt.Tx, err error) {
	txData, err := LoadTxData(txid)
	if err != nil {
		return
	}
	return bt.NewTxFromBytes(txData.Transaction)
}

func LoadTxData(txid []byte) (*models.Transaction, error) {
	key := base64.StdEncoding.EncodeToString(txid)
	if txData, ok := txCache.Get(key); ok {
		return txData, nil
	}
	fmt.Printf("Fetching Tx: %x\n", txid)
	txData, err := JBClient.GetTransaction(context.Background(), hex.EncodeToString(txid))
	if err != nil {
		return nil, err
	}
	txCache.Add(key, txData)
	return txData, nil
}

// ByteString is a byte array that serializes to hex
type ByteString []byte

// MarshalJSON serializes ByteArray to hex
func (s ByteString) MarshalJSON() ([]byte, error) {
	bytes, err := json.Marshal(fmt.Sprintf("%x", string(s)))
	return bytes, err
}

// UnmarshalJSON deserializes ByteArray to hex
func (s *ByteString) UnmarshalJSON(data []byte) error {
	var x string
	err := json.Unmarshal(data, &x)
	if err == nil {
		str, e := hex.DecodeString(x)
		*s = ByteString([]byte(str))
		err = e
	}

	return err
}
