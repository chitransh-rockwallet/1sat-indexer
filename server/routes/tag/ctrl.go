package tag

import (
	"strings"

	"github.com/chitransh-rockwallet/1sat-indxer/config"
	"github.com/chitransh-rockwallet/1sat-indxer/evt"
	"github.com/chitransh-rockwallet/1sat-indxer/idx"
	"github.com/gofiber/fiber/v2"
)

var ingest *idx.IngestCtx

func RegisterRoutes(r fiber.Router, ingestCtx *idx.IngestCtx) {
	ingest = ingestCtx
	r.Get("/:tag", TxosByTag)
}

func TxosByTag(c *fiber.Ctx) error {

	tags := strings.Split(c.Query("tags", ""), ",")
	if len(tags) > 0 && tags[0] == "*" {
		tags = ingest.IndexedTags()
	}

	from := c.QueryFloat("from", 0)
	if txos, err := config.Store.SearchTxos(c.Context(), &idx.SearchCfg{
		Keys:          []string{evt.TagKey(c.Params("tag"))},
		From:          &from,
		Reverse:       c.QueryBool("rev", false),
		Limit:         uint32(c.QueryInt("limit", 100)),
		IncludeTxo:    c.QueryBool("txo", false),
		IncludeTags:   tags,
		IncludeScript: c.QueryBool("script", false),
	}); err != nil {
		return err
	} else {
		return c.JSON(txos)
	}
}
