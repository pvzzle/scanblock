//go:build integration

package pg_test

import (
	"context"
	"os"
	"testing"
	"time"

	"scanblock/internal/storage"
	"scanblock/internal/storage/pg"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepo_UpsertAndHistory(t *testing.T) {
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		dsn = os.Getenv("PG_DSN")
	}
	if dsn == "" {
		t.Skip("TEST_PG_DSN/PG_DSN is not set")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	repo := pg.New(pool)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	// чистим после миграций (быстро и предсказуемо)
	_, _ = pool.Exec(ctx, "TRUNCATE chat_tx, transactions RESTART IDENTITY CASCADE")

	now := time.Now().UTC()
	bn := uint64(123)
	st := uint8(1)
	to := "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	gp := "1"

	tx := storage.TxRecord{
		Hash:        "0x" + repeat("1", 64),
		ChainID:     "1",
		BlockNum:    &bn,
		BlockTime:   &now,
		FromAddr:    "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ToAddr:      &to,
		ValueWei:    "1000000000000000000",
		Nonce:       0,
		TxType:      0,
		Gas:         21000,
		GasPriceWei: &gp,
		Status:      &st,
	}

	if err := repo.UpsertTx(ctx, tx); err != nil {
		t.Fatalf("UpsertTx: %v", err)
	}

	chatID := int64(42)
	if err := repo.AddChatEvent(ctx, chatID, tx.Hash, storage.EventSearch); err != nil {
		t.Fatalf("AddChatEvent: %v", err)
	}

	h, err := repo.ListHistory(ctx, chatID, 10)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(h) != 1 {
		t.Fatalf("expected 1 history item, got=%d", len(h))
	}
	if h[0].Hash != tx.Hash {
		t.Fatalf("expected hash=%s got=%s", tx.Hash, h[0].Hash)
	}
	if h[0].EventType != storage.EventSearch {
		t.Fatalf("expected event=search got=%s", h[0].EventType)
	}
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
