package tx

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log"
	"strings"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/chitransh-rockwallet/1sat-indexer/blk"
	"github.com/chitransh-rockwallet/1sat-indexer/broadcast"
	"github.com/chitransh-rockwallet/1sat-indexer/evt"
	"github.com/chitransh-rockwallet/1sat-indexer/idx"
	"github.com/chitransh-rockwallet/1sat-indexer/jb"
	"github.com/gofiber/fiber/v2"
)

var ingest *idx.IngestCtx
var b transaction.Broadcaster

func RegisterRoutes(r fiber.Router, ingestCtx *idx.IngestCtx, broadcaster transaction.Broadcaster) {
	ingest = ingestCtx
	b = broadcaster
	r.Post("/", BroadcastTx)
	r.Get("/:txid", GetTxWithProof)
	r.Get("/:txid/raw", GetRawTx)
	r.Get("/:txid/proof", GetProof)
	r.Get("/:txid/txos", TxosByTxid)
	r.Get("/:txid/beef", GetTxBEEF)
	r.Post("/callback", TxCallback)
	r.Get("/:txid/parse", ParseTx)
	r.Post("/parse", ParseTx)
	r.Post("/:txid/ingest", IngestTx)
}

func BroadcastTx(c *fiber.Ctx) (err error) {
	var tx *transaction.Transaction
	mime := c.Get("Content-Type")
	switch mime {
	case "application/octet-stream":
		if c.Query("fmt") == "beef" {
			if tx, err = transaction.NewTransactionFromBEEF(c.Body()); err != nil {
				return c.SendStatus(400)
			}
		} else {
			if tx, err = transaction.NewTransactionFromBytes(c.Body()); err != nil {
				return c.SendStatus(400)
			}
		}
	case "text/plain":
		if c.Query("fmt") == "beef" {
			if tx, err = transaction.NewTransactionFromBEEFHex(string(c.Body())); err != nil {
				return c.SendStatus(400)
			}
		} else {
			if tx, err = transaction.NewTransactionFromHex(string(c.Body())); err != nil {
				return c.SendStatus(400)
			}
		}
	default:
		return c.SendStatus(400)
	}

	response := broadcast.Broadcast(c.Context(), ingest.Store, tx, b)
	if response.Success {
		if _, err := ingest.IngestTx(c.Context(), tx, idx.AncestorConfig{Load: true, Parse: true, Save: true}); err != nil {
			log.Println("Ingest Error", tx.TxID().String(), err)
			// } else if out, err := json.MarshalIndent(idxCtx, "", "  "); err != nil {
			// 	log.Println("Ingest Error", tx.TxID().String(), err)
			// } else {
			// 	log.Println("Ingest", tx.TxID().String(), string(out))
		}
		evt.Publish(c.Context(), "broadcast", base64.StdEncoding.EncodeToString(tx.Bytes()))
	} else {
		log.Println("Broadcast Error", tx.TxID().String(), response.Error)
	}
	c.Status(int(response.Status))
	return c.JSON(response)

}

func GetTxBEEF(c *fiber.Ctx) error {
	if tx, err := jb.BuildTxBEEF(c.Context(), c.Params("txid")); err != nil {
		if err == jb.ErrNotFound {
			return c.SendStatus(404)
		} else {
			return err
		}
	} else if beef, err := tx.AtomicBEEF(true); err != nil {
		return err
	} else {
		c.Set("Content-Type", "application/octet-stream")
		return c.Send(beef)
	}
}

func GetTxWithProof(c *fiber.Ctx) error {
	txid := c.Params("txid")
	if rawtx, err := jb.LoadRawtx(c.Context(), txid); err != nil {
		if err == jb.ErrNotFound {
			return c.SendStatus(404)
		}
		return err
	} else if len(rawtx) == 0 {
		return c.SendStatus(404)
	} else {
		c.Set("Content-Type", "application/octet-stream")
		buf := bytes.NewBuffer([]byte{})
		buf.Write(transaction.VarInt(uint64(len(rawtx))).Bytes())
		buf.Write(rawtx)
		if proof, err := jb.LoadProof(c.Context(), txid); err != nil {
			return err
		} else {
			if proof == nil {
				buf.Write(transaction.VarInt(0).Bytes())
				c.Set("Cache-Control", "public,max-age=60")
			} else if chaintip, err := blk.GetChaintip(c.Context()); err != nil {
				return err
			} else {
				if proof.BlockHeight < chaintip.Height-5 {
					c.Set("Cache-Control", "public,max-age=31536000,immutable")
				} else {
					c.Set("Cache-Control", "public,max-age=60")
				}
				bin := proof.Bytes()
				buf.Write(transaction.VarInt(uint64(len(bin))).Bytes())
				buf.Write(bin)
			}
		}
		return c.Send(buf.Bytes())
	}
}

func GetRawTx(c *fiber.Ctx) error {
	txid := c.Params("txid")
	if rawtx, err := jb.LoadRawtx(c.Context(), txid); err != nil {
		return err
	} else if rawtx == nil {
		return c.SendStatus(404)
	} else {
		c.Set("Cache-Control", "public,max-age=31536000,immutable")
		switch c.Query("fmt", "bin") {
		case "json":
			if tx, err := transaction.NewMerklePathFromBinary(rawtx); err != nil {
				return err
			} else {
				return c.JSON(tx)
			}
		case "hex":
			c.Set("Content-Type", "text/plain")
			return c.SendString(hex.EncodeToString(rawtx))
		default:
			c.Set("Content-Type", "application/octet-stream")
			return c.Send(rawtx)
		}
	}
}

func GetProof(c *fiber.Ctx) error {
	txid := c.Params("txid")
	if proof, err := jb.LoadProof(c.Context(), txid); err != nil {
		return err
	} else if proof == nil {
		return c.SendStatus(404)
	} else if chaintip, err := blk.GetChaintip(c.Context()); err != nil {
		return err
	} else {
		if proof.BlockHeight < chaintip.Height-5 {
			c.Set("Cache-Control", "public,max-age=31536000,immutable")
		} else {
			c.Set("Cache-Control", "public,max-age=60")
		}
		switch c.Query("fmt", "bin") {
		case "json":
			return c.JSON(proof)
		case "hex":
			c.Set("Content-Type", "text/plain")
			return c.SendString(proof.Hex())
		default:
			c.Set("Content-Type", "application/octet-stream")
			return c.Send(proof.Bytes())
		}
	}
}

func TxCallback(c *fiber.Ctx) error {
	l := make(map[string]any)
	l["headers"] = c.GetReqHeaders()
	l["body"] = string(c.BodyRaw())
	if out, err := json.MarshalIndent(l, "", "  "); err != nil {
		return err
	} else {
		log.Println("TxCallback", string(out))
		return c.SendStatus(200)
	}
}

func ParseTx(c *fiber.Ctx) (err error) {
	txid := c.Params("txid")
	var tx *transaction.Transaction
	var rawtx []byte
	if txid != "" {
		if tx, err = jb.LoadTx(c.Context(), txid, false); err != nil {
			return
		} else if tx == nil {
			return c.SendStatus(404)
		}
	} else if err = c.BodyParser(rawtx); err != nil {
		return
	} else if len(rawtx) == 0 {
		return c.SendStatus(400)
	} else if tx, err = transaction.NewTransactionFromBytes(rawtx); err != nil {
		return
	}
	if idxCtx, err := ingest.ParseTx(c.Context(), tx, idx.AncestorConfig{Load: true, Parse: true, Save: true}); err != nil {
		return err
	} else {
		return c.JSON(idxCtx)
	}
}

func IngestTx(c *fiber.Ctx) error {
	txid := c.Params("txid")
	if tx, err := jb.LoadTx(c.Context(), txid, true); err != nil {
		return err
	} else if tx == nil {
		return c.SendStatus(404)
	} else if idxCtx, err := ingest.IngestTx(c.Context(), tx, idx.AncestorConfig{Load: true, Parse: true}); err != nil {
		return err
	} else {
		return c.JSON(idxCtx)
	}
}

func TxosByTxid(c *fiber.Ctx) error {
	txid := c.Params("txid")

	tags := strings.Split(c.Query("tags", ""), ",")
	if len(tags) > 0 && tags[0] == "*" {
		tags = ingest.IndexedTags()
	}
	if txos, err := ingest.Store.LoadTxosByTxid(c.Context(), txid, tags, c.QueryBool("script", false), c.QueryBool("spend", false)); err != nil {
		return err
	} else {
		return c.JSON(txos)
	}
}
