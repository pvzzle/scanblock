package storage

import "context"

type Repository interface {
	EnsureSchema(ctx context.Context) error

	UpsertTx(ctx context.Context, tx TxRecord) error
	AddChatEvent(ctx context.Context, chatID int64, txHash string, eventType TxEventType) error

	ListHistory(ctx context.Context, chatID int64, limit int) ([]HistoryItem, error)
}
