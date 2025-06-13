package idx

import (
	"encoding/json"
	"log"
)

type Indexer interface {
	Tag() string
	Parse(idxCtx *IndexContext, vout uint32) *IndexData
	PreSave(idxCtx *IndexContext)
	FromBytes(data []byte) (obj any, err error)
	// Bytes(obj any) ([]byte, error)
	// PostProcess(ctx context.Context, outpoint *Outpoint) error
}

type BaseIndexer struct{}

func (b BaseIndexer) Tag() string {
	return ""
}

func (b BaseIndexer) Parse(idxCtx *IndexContext, vout uint32) (idxData *IndexData) {
	return
}

func (b BaseIndexer) PreSave(idxCtx *IndexContext) {}

func (b BaseIndexer) FromBytes(data []byte) (any, error) {
	if len(data) == 0 {
		return nil, nil
	}
	obj := make(map[string]any)
	if err := json.Unmarshal(data, &obj); err != nil {
		log.Panicf("Error unmarshalling: %s %v", b.Tag(), err)
		return nil, err
	}
	return obj, nil
}

// func (b BaseIndexer) Bytes(obj any) ([]byte, error) {
// 	return json.Marshal(obj)
// }

// func (b BaseIndexer) PostProcess(ctx context.Context, outpoint *Outpoint) error {
// 	return nil
// }
