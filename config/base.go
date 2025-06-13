package config

import (
	"github.com/bsv-blockchain/go-sdk/transaction/broadcaster"
	"github.com/chitransh-rockwallet/1sat-indexer/idx"
	"github.com/chitransh-rockwallet/1sat-indexer/lib"
)

var Indexers = []idx.Indexer{}
var Broadcaster *broadcaster.Arc
var Network = lib.Mainnet
var Store idx.TxoStore
