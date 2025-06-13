package config

import (
	"github.com/bsv-blockchain/go-sdk/transaction/broadcaster"
	"github.com/chitransh-rockwallet/1sat-indxer/idx"
	"github.com/chitransh-rockwallet/1sat-indxer/lib"
)

var Indexers = []idx.Indexer{}
var Broadcaster *broadcaster.Arc
var Network = lib.Mainnet
var Store idx.TxoStore
