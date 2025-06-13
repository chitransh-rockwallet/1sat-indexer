package onesat

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/chitransh-rockwallet/1sat-indexer/evt"
	"github.com/chitransh-rockwallet/1sat-indexer/idx"
	"github.com/chitransh-rockwallet/1sat-indexer/lib"
)

const BSV21_INDEX_FEE = 1000
const BSV21_TAG = "bsv21"

var (
	IssueEvent   string = "iss"
	IdEvent      string = "id"
	ValidEvent   string = "val"
	InvalidEvent string = "inv"
	PendingEvent string = "pen"
)

type Bsv21 struct {
	Id          string      `json:"id,omitempty"`
	Op          string      `json:"op"`
	Symbol      *string     `json:"sym,omitempty"`
	Decimals    uint8       `json:"dec"`
	Icon        string      `json:"icon,omitempty"`
	Amt         uint64      `json:"amt"`
	Status      Bsv20Status `json:"status"`
	Reason      *string     `json:"reason,omitempty"`
	FundAddress string      `json:"fundAddress,omitempty"`
	FundBalance int         `json:"-"`
}

type Bsv21Indexer struct {
	idx.BaseIndexer
	WhitelistFn *func(tokenId string) bool
	BlacklistFn *func(tokenId string) bool
}

func (i *Bsv21Indexer) Tag() string {
	return BSV21_TAG
}

func (i *Bsv21Indexer) FromBytes(data []byte) (any, error) {
	return Bsv21FromBytes(data)
}

func Bsv21FromBytes(data []byte) (*Bsv21, error) {
	obj := &Bsv21{}
	if err := json.Unmarshal(data, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func (i *Bsv21Indexer) Parse(idxCtx *idx.IndexContext, vout uint32) *idx.IndexData {
	txo := idxCtx.Txos[vout]

	var err error
	if idxData, ok := txo.Data[INSC_TAG]; !ok {
		return nil
	} else if insc, ok := idxData.Data.(*Inscription); !ok {
		return nil
	} else if insc.JsonMap == nil || insc.File == nil || insc.File.Type != "application/bsv-20" {
		return nil
	} else if protocol, ok := insc.JsonMap["p"]; !ok || protocol != "bsv-20" {
		return nil
	} else {
		// if i.WhitelistFn != nil {
		// 	whitelisted := (*i.WhitelistFn)(id)
		// 	if !whitelisted {
		// 		return nil
		// 	}

		// }
		// if i.BlacklistFn != nil {
		// 	blacklisted := (*i.BlacklistFn)(id)
		// 	if blacklisted {
		// 		return nil
		// 	}
		// }
		bsv21 := &Bsv21{}
		if op, ok := insc.JsonMap["op"]; ok {
			bsv21.Op = strings.ToLower(op)
		} else {
			return nil
		}

		if amt, ok := insc.JsonMap["amt"]; ok {
			if bsv21.Amt, err = strconv.ParseUint(amt, 10, 64); err != nil {
				return nil
			}
		}

		if dec, ok := insc.JsonMap["dec"]; ok {
			var val uint64
			if val, err = strconv.ParseUint(dec, 10, 8); err != nil || val > 18 {
				return nil
			}
			bsv21.Decimals = uint8(val)
		}

		events := []*evt.Event{}

		switch bsv21.Op {
		case "deploy+mint":
			bsv21.Id = txo.Outpoint.String()
			if sym, ok := insc.JsonMap["sym"]; ok {
				bsv21.Symbol = &sym
			}
			bsv21.Status = Valid
			if icon, ok := insc.JsonMap["icon"]; ok {
				if strings.HasPrefix(icon, "_") {
					icon = fmt.Sprintf("%x%s", txo.Outpoint.Txid(), icon)
				}
				bsv21.Icon = icon
			}
			bsv21.Id = txo.Outpoint.String()
			hash := sha256.Sum256(*txo.Outpoint)
			path := fmt.Sprintf("21/%d/%d", binary.BigEndian.Uint32(hash[:8])>>1, binary.BigEndian.Uint32(hash[24:])>>1)
			ek, err := idxHdKey.DeriveChildFromPath(path)
			if err != nil {
				log.Panic(err)
			}
			pubKey, err := ek.ECPubKey()
			if err != nil {
				log.Panic(err)
			}
			if add, err := script.NewAddressFromPublicKey(pubKey, idxCtx.Network == lib.Mainnet); err == nil {
				bsv21.FundAddress = add.AddressString
			}

			events = append(events, &evt.Event{
				Id:    IssueEvent,
				Value: "",
			})
		case "transfer", "burn":
			if id, ok := insc.JsonMap["id"]; !ok {
				return nil
			} else if _, err = lib.NewOutpointFromString(id); err != nil {
				return nil
			} else {
				bsv21.Id = id
			}
		default:
			return nil
		}

		events = append(events, &evt.Event{
			Id:    IdEvent,
			Value: bsv21.Id,
		})
		return &idx.IndexData{
			Data:   bsv21,
			Events: events,
		}
	}
}

type bsv21Token struct {
	balance uint64
	reason  *string
	token   *Bsv21
	outputs []*idx.IndexData
	deps    []*lib.Outpoint
}
type bsv21Ctx struct {
	tokens map[string]*bsv21Token
}

func (i *Bsv21Indexer) PreSave(idxCtx *idx.IndexContext) {
	ctx := bsv21Ctx{
		tokens: map[string]*bsv21Token{},
	}

	isPending := false
	for _, txo := range idxCtx.Txos {
		if idxData, ok := txo.Data[BSV21_TAG]; ok {
			if bsv21, ok := idxData.Data.(*Bsv21); ok {
				if bsv21.Op == "deploy+mint" {
					continue
				}
				if token, ok := ctx.tokens[bsv21.Id]; !ok {
					token = &bsv21Token{
						outputs: []*idx.IndexData{
							idxData,
						},
					}
					ctx.tokens[bsv21.Id] = token
				} else {
					token.outputs = append(token.outputs, idxData)
				}
			}
		}
	}
	if len(ctx.tokens) == 0 {
		return
	}

	for _, spend := range idxCtx.Spends {
		if spend.Satoshis == nil {
			isPending = true
			break
		}
		if idxData, ok := spend.Data[BSV21_TAG]; ok {
			if bsv21, ok := idxData.Data.(*Bsv21); ok {
				if bsv21.Status == Pending {
					isPending = true
					break
				} else if bsv21.Status == Valid {
					if token, ok := ctx.tokens[bsv21.Id]; ok {
						token.balance += bsv21.Amt
						token.token = bsv21
						token.deps = append(token.deps, spend.Outpoint)
					}
				}
			}
		}
	}

	reasons := map[string]string{}
	if !isPending {
		for _, token := range ctx.tokens {
			for _, idxData := range token.outputs {
				if bsv21, ok := idxData.Data.(*Bsv21); ok {
					if token.token == nil {
						reason := "missing inputs"
						token.reason = &reason
					} else if bsv21.Amt > token.balance {
						reason := "insufficient funds"
						token.reason = &reason
					} else {
						bsv21.Icon = token.token.Icon
						bsv21.Symbol = token.token.Symbol
						bsv21.Decimals = token.token.Decimals
						bsv21.FundAddress = token.token.FundAddress
						token.balance -= bsv21.Amt
					}
				}
			}
		}
	}
	for tokenId, token := range ctx.tokens {
		for _, idxData := range token.outputs {
			if bsv21, ok := idxData.Data.(*Bsv21); ok {
				if isPending {
					idxData.Events = append(idxData.Events, &evt.Event{
						Id:    PendingEvent,
						Value: tokenId,
					})
				} else if reason, ok := reasons[tokenId]; ok {
					bsv21.Status = Invalid
					bsv21.Reason = &reason
					idxData.Events = append(idxData.Events, &evt.Event{
						Id:    InvalidEvent,
						Value: tokenId,
					})
					idxData.Deps = append(idxData.Deps, token.deps...)
				} else {
					bsv21.Status = Valid
					idxData.Events = append(idxData.Events, &evt.Event{
						Id:    ValidEvent,
						Value: tokenId,
					})
					idxData.Deps = append(idxData.Deps, token.deps...)
				}
			}
		}
	}
}
