package txos

import (
	"strings"

	"github.com/chitransh-rockwallet/1sat-indexer/idx"
	"github.com/gofiber/fiber/v2"
)

var ingest *idx.IngestCtx

func RegisterRoutes(r fiber.Router, ingestCtx *idx.IngestCtx) {
	ingest = ingestCtx
	r.Get("/:outpoint", GetTxo)
	r.Post("/", GetTxos)
}

func GetTxo(c *fiber.Ctx) error {
	tags := strings.Split(c.Query("tags", ""), ",")
	if len(tags) > 0 && tags[0] == "*" {
		tags = ingest.IndexedTags()
	}
	if txo, err := ingest.Store.LoadTxo(c.Context(), c.Params("outpoint"), tags, c.QueryBool("script", false), c.QueryBool("spend", false)); err != nil {
		return err
	} else if txo == nil {
		return c.SendStatus(404)
	} else {
		c.Set("Cache-Control", "public,max-age=60")
		return c.JSON(txo)
	}
}

func GetTxos(c *fiber.Ctx) error {
	var outpoints []string
	if err := c.BodyParser(&outpoints); err != nil {
		return err
	}
	tags := strings.Split(c.Query("tags", ""), ",")
	if len(tags) > 0 && tags[0] == "*" {
		tags = ingest.IndexedTags()
	}
	if txos, err := ingest.Store.LoadTxos(c.Context(), outpoints, tags, c.QueryBool("script", false), c.QueryBool("spend", false)); err != nil {
		return err
	} else {
		c.Set("Cache-Control", "public,max-age=60")
		return c.JSON(txos)
	}
}
