package storage

import "time"

type TxRecord struct {
	Hash        string
	ChainID     string
	BlockNum    *uint64
	BlockTime   *time.Time
	FromAddr    string
	ToAddr      *string
	ValueWei    string // big.Int как строка
	Nonce       uint64
	TxType      uint8
	Gas         uint64
	GasPriceWei *string // может быть nil для некоторых tx
	Status      *uint8  // 1 success, 0 failed, nil unknown/pending
}

type TxEventType string

const (
	EventSearch TxEventType = "search"
	EventNotify TxEventType = "notify"
)

type HistoryItem struct {
	At        time.Time
	EventType TxEventType

	Hash      string
	BlockNum  *uint64
	BlockTime *time.Time
	FromAddr  string
	ToAddr    *string
	ValueWei  string
	Status    *uint8
}
