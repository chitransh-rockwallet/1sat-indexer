package evt

import (
	"net/url"
	"strings"

	"github.com/chitransh-rockwallet/1sat-indexer/evt"
	"github.com/chitransh-rockwallet/1sat-indexer/idx"
	"github.com/gofiber/fiber/v2"
)

var ingest *idx.IngestCtx

func RegisterRoutes(r fiber.Router, ingestCtx *idx.IngestCtx) {
	ingest = ingestCtx
	r.Get("/:tag/:id/:value", TxosByEvent)
}

func TxosByEvent(c *fiber.Ctx) error {
	tags := strings.Split(c.Query("tags", ""), ",")
	if len(tags) > 0 && tags[0] == "*" {
		tags = ingest.IndexedTags()
	}

	decodedValue, _ := url.QueryUnescape(c.Params("value"))
	from := c.QueryFloat("from", 0)
	if txos, err := ingest.Store.SearchTxos(c.Context(), &idx.SearchCfg{
		Keys: []string{evt.EventKey(c.Params("tag"), &evt.Event{
			Id:    c.Params("id"),
			Value: decodedValue,
		})},
		From:          &from,
		Reverse:       c.QueryBool("rev", false),
		Limit:         uint32(c.QueryInt("limit", 100)),
		IncludeTxo:    c.QueryBool("txo", false),
		IncludeTags:   tags,
		IncludeScript: c.QueryBool("script", false),
		IncludeSpend:  c.QueryBool("spend", false),
		FilterSpent:   c.QueryBool("unspent", false),
	}); err != nil {
		return err
	} else {
		return c.JSON(txos)
	}
}
