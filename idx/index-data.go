package idx

import (
	"encoding/json"

	"github.com/chitransh-rockwallet/1sat-indexer/evt"
	"github.com/chitransh-rockwallet/1sat-indexer/lib"
)

type IndexData struct {
	Data   any
	Events []*evt.Event
	Deps   []*lib.Outpoint
}

func (id IndexData) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.Data)
}

func (id *IndexData) UnmarshalJSON(data []byte) error {
	id.Data = json.RawMessage([]byte{})
	return json.Unmarshal(data, &id.Data)
}
