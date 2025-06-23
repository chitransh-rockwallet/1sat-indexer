package bitcom

import (
	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/chitransh-rockwallet/1sat-indexer/v5/idx"
)

const BITCOM_TAG = "bitcom"

type Bitcom struct {
	Protocol string
	Script   []byte
	Pos      int
}

type BitcomIndexer struct {
	idx.BaseIndexer
}

func (i *BitcomIndexer) Tag() string {
	return BITCOM_TAG
}

var B = "19HxigV4QyBv3tHpQVcUEQyq1pzZVdoAut"

func (i *BitcomIndexer) Parse(idxCtx *idx.IndexContext, vout uint32) *idx.IndexData {
	scr := idxCtx.Tx.Outputs[vout].LockingScript

	var bitcom []*Bitcom

	start := 0
	var opReturn int
	for i := start; i < len(*scr); {
		startI := i
		op, err := scr.ReadOp(&i)
		if err != nil {
			break
		}
		switch op.Op {
		case script.OpRETURN:
			if opReturn == 0 {
				opReturn = startI
			} else {
				continue
			}
			if op, err := scr.ReadOp(&i); err != nil {
				continue
			} else if len(op.Data) > 0 {
				bitcom = append(bitcom, &Bitcom{
					Protocol: string(op.Data),
					Script:   (*scr)[i:],
					Pos:      i,
				})
			}

		case script.OpDATA1:
			if opReturn > 0 && op.Data[0] == '|' {
				if op, err := scr.ReadOp(&i); err != nil {
					continue
				} else {
					bitcom = append(bitcom, &Bitcom{
						Protocol: string(op.Data),
						Script:   (*scr)[i:],
						Pos:      i,
					})
				}
			}
		}
	}
	return &idx.IndexData{
		Data: bitcom,
	}
}

func (i *BitcomIndexer) PreSave(idxCtx *idx.IndexContext) {
	for _, txo := range idxCtx.Txos {
		if txo.Data[BITCOM_TAG] != nil {
			delete(txo.Data, BITCOM_TAG)
		}
	}
}
