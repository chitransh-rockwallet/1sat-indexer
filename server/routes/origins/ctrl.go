package origins

import (
	"encoding/json"
	"strings"

	"github.com/chitransh-rockwallet/1sat-indexer/evt"
	"github.com/chitransh-rockwallet/1sat-indexer/idx"
	"github.com/chitransh-rockwallet/1sat-indexer/mod/onesat"
	"github.com/gofiber/fiber/v2"
)

var ingest *idx.IngestCtx

func RegisterRoutes(r fiber.Router, ingestCtx *idx.IngestCtx) {
	ingest = ingestCtx
	r.Post("/ancestors", OriginsAncestors)
	r.Get("/ancestors/:outpoint", OriginAncestors)
	r.Post("/history", OriginsHistory)
	r.Get("/history/:outpoint", OriginsHistory)
}

func OriginHistory(c *fiber.Ctx) error {
	outpoint := c.Params("outpoint")
	tags := strings.Split(c.Query("tags", ""), ",")
	if len(tags) > 0 && tags[0] == "*" {
		tags = ingest.IndexedTags()
	}

	if txos, err := ingest.Store.SearchTxos(c.Context(), &idx.SearchCfg{
		Keys: []string{evt.EventKey("origin", &evt.Event{
			Id:    "outpoint",
			Value: outpoint,
		})},
		IncludeTxo:    c.QueryBool("txo", false),
		IncludeTags:   tags,
		IncludeScript: c.QueryBool("script", false),
	}); err != nil {
		return err
	} else {
		return c.JSON(txos)
	}

}

func OriginsHistory(c *fiber.Ctx) error {
	var outpoints []string
	if err := c.BodyParser(&outpoints); err != nil {
		return c.SendStatus(400)
	} else if len(outpoints) == 0 {
		return c.SendStatus(400)
	}

	tags := strings.Split(c.Query("tags", ""), ",")
	if len(tags) > 0 && tags[0] == "*" {
		tags = ingest.IndexedTags()
	}

	history := make([]*idx.Txo, 0, len(outpoints))
	for _, outpoint := range outpoints {
		if txos, err := ingest.Store.SearchTxos(c.Context(), &idx.SearchCfg{
			Keys: []string{evt.EventKey("origin", &evt.Event{
				Id:    "outpoint",
				Value: outpoint,
			})},
			IncludeTxo:    c.QueryBool("txo", false),
			IncludeTags:   tags,
			IncludeScript: c.QueryBool("script", false),
		}); err != nil {
			return err
		} else {
			history = append(history, txos...)
		}
	}
	return c.JSON(history)
}

func OriginAncestors(c *fiber.Ctx) error {
	outpoint := c.Params("outpoint")

	tags := strings.Split(c.Query("tags", ""), ",")
	if len(tags) > 0 && tags[0] == "*" {
		tags = ingest.IndexedTags()
	}

	if data, err := ingest.Store.LoadData(c.Context(), outpoint, []string{"origin"}); err != nil {
		return err
	} else {
		origin := onesat.Origin{}
		if err := json.Unmarshal([]byte(data["origin"].Data.(json.RawMessage)), &origin); err != nil {
			return err
		}
		outpoint = origin.Outpoint.String()
	}

	ancestors := make([]*idx.Txo, 0)
	if txos, err := ingest.Store.SearchTxos(c.Context(), &idx.SearchCfg{
		Keys: []string{evt.EventKey("origin", &evt.Event{
			Id:    "outpoint",
			Value: outpoint,
		})},
		IncludeTxo:    c.QueryBool("txo", false),
		IncludeTags:   tags,
		IncludeScript: c.QueryBool("script", false),
	}); err != nil {
		return err
	} else {
		for _, txo := range txos {
			if txo.Outpoint.String() != outpoint {
				ancestors = append(ancestors, txo)
			}
		}
	}
	return c.JSON(ancestors)
}

func OriginsAncestors(c *fiber.Ctx) error {
	var outpoints []string
	if err := c.BodyParser(&outpoints); err != nil {
		return c.SendStatus(400)
	} else if len(outpoints) == 0 {
		return c.SendStatus(400)
	}

	tags := strings.Split(c.Query("tags", ""), ",")
	if len(tags) > 0 && tags[0] == "*" {
		tags = ingest.IndexedTags()
	}

	origins := make([]string, 0, len(outpoints))
	outpointMap := make(map[string]struct{}, len(outpoints))
	for _, outpoint := range outpoints {
		if data, err := ingest.Store.LoadData(c.Context(), outpoint, []string{"origin"}); err != nil {
			return err
		} else {
			origin := onesat.Origin{}
			if err := json.Unmarshal([]byte(data["origin"].Data.(json.RawMessage)), &origin); err != nil {
				return err
			}
			if origin.Outpoint != nil {
				origins = append(origins, origin.Outpoint.String())
			}
		}
	}

	ancestors := make([]*idx.Txo, 0, len(origins))
	for _, outpoint := range origins {
		if txos, err := ingest.Store.SearchTxos(c.Context(), &idx.SearchCfg{
			Keys: []string{evt.EventKey("origin", &evt.Event{
				Id:    "outpoint",
				Value: outpoint,
			})},
			IncludeTxo:    c.QueryBool("txo", false),
			IncludeTags:   tags,
			IncludeScript: c.QueryBool("script", false),
		}); err != nil {
			return err
		} else {
			for _, txo := range txos {
				op := txo.Outpoint.String()
				if _, ok := outpointMap[op]; !ok {
					outpointMap[op] = struct{}{}
					ancestors = append(ancestors, txo)
				}
			}
		}
	}

	return c.JSON(ancestors)
}
