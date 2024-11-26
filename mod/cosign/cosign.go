package cosign

import (
	"encoding/hex"
	"encoding/json"

	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/shruggr/1sat-indexer/v5/evt"
	"github.com/shruggr/1sat-indexer/v5/idx"
)

const COSIGN_TAG = "cosign"

type Cosign struct {
	Address  string `json:"address"`
	Cosigner string `json:"cosigner"`
}

type CosignIndexer struct {
	idx.BaseIndexer
}

func (i *CosignIndexer) Tag() string {
	return COSIGN_TAG
}

func (i *CosignIndexer) FromBytes(data []byte) (any, error) {
	return CosignFromBytes(data)
}

func CosignFromBytes(data []byte) (*Cosign, error) {
	obj := &Cosign{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func (i *CosignIndexer) Parse(idxCtx *idx.IndexContext, vout uint32) *idx.IndexData {
	txo := idxCtx.Txos[vout]

	cosign := parseScript(idxCtx.Tx.Outputs[vout].LockingScript)
	if cosign != nil {
		txo.AddOwner(cosign.Address)
		return &idx.IndexData{
			Data: cosign,
			Events: []*evt.Event{
				{
					Id:    "own",
					Value: cosign.Address,
				},
				{
					Id:    "cosigner",
					Value: cosign.Cosigner,
				},
			},
		}
	}
	return nil
}

func parseScript(s *script.Script) *Cosign {
	chunks, _ := s.Chunks()
	for i := range len(chunks) - 6 {
		if chunks[0+i].Op == script.OpDUP &&
			chunks[1+i].Op == script.OpHASH160 &&
			len(chunks[2+i].Data) == 20 &&
			chunks[3+i].Op == script.OpEQUALVERIFY &&
			chunks[4+i].Op == script.OpCHECKSIGVERIFY &&
			len(chunks[5+i].Data) == 33 &&
			chunks[6+i].Op == script.OpCHECKSIG {

			cosign := &Cosign{
				Cosigner: hex.EncodeToString(chunks[5+i].Data),
			}
			if add, err := script.NewAddressFromPublicKeyHash(chunks[2+i].Data, true); err == nil {
				cosign.Address = add.AddressString
			}
			return cosign
		}
	}
	return nil
}
