package spend

import (
	"github.com/chitransh-rockwallet/1sat-indxer/idx"
	"github.com/gofiber/fiber/v2"
)

var ingest *idx.IngestCtx

func RegisterRoutes(r fiber.Router, ingestCtx *idx.IngestCtx) {
	ingest = ingestCtx
	r.Get("/:outpoint", GetSpend)
	r.Post("/", GetSpends)
}

func GetSpend(c *fiber.Ctx) error {
	outpoint := c.Params("outpoint")
	if spend, err := ingest.Store.GetSpends(c.Context(), []string{outpoint}, true); err != nil {
		return err
	} else {
		return c.JSON(spend)
	}
}

func GetSpends(c *fiber.Ctx) error {
	outpoints := []string{}
	if err := c.BodyParser(&outpoints); err != nil {
		return err
	}
	if spends, err := ingest.Store.GetSpends(c.Context(), outpoints, true); err != nil {
		return err
	} else {
		return c.JSON(spends)
	}
}
