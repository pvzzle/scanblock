package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/pvzzle/scanblock/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

func (r *Postgres) EnsureSchema(ctx context.Context) error {
	ddl := `
CREATE TABLE IF NOT EXISTS transactions (
  hash TEXT PRIMARY KEY,
  chain_id TEXT NOT NULL,

  block_number BIGINT NULL,
  block_time TIMESTAMPTZ NULL,

  from_addr TEXT NOT NULL,
  to_addr   TEXT NULL,

  value_wei NUMERIC(78,0) NOT NULL,
  nonce     BIGINT NOT NULL,
  tx_type   INT NOT NULL,
  gas       BIGINT NOT NULL,
  gas_price_wei NUMERIC(78,0) NULL,

  status SMALLINT NULL, -- 1 success, 0 failed

  first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS chat_tx (
  chat_id BIGINT NOT NULL,
  tx_hash TEXT NOT NULL REFERENCES transactions(hash) ON DELETE CASCADE,
  event_type TEXT NOT NULL, -- search|notify
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (chat_id, tx_hash, event_type)
);

CREATE INDEX IF NOT EXISTS chat_tx_chat_created_idx ON chat_tx(chat_id, created_at DESC);
`
	_, err := r.pool.Exec(ctx, ddl)
	return err
}

func (r *Postgres) UpsertTx(ctx context.Context, tx storage.TxRecord) error {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var (
		blockNum  any = nil
		blockTime any = nil
		toAddr    any = nil
		gasPrice  any = nil
		status    any = nil
	)

	if tx.BlockNum != nil {
		blockNum = int64(*tx.BlockNum)
	}
	if tx.BlockTime != nil {
		blockTime = *tx.BlockTime
	}
	if tx.ToAddr != nil {
		toAddr = *tx.ToAddr
	}
	if tx.GasPriceWei != nil {
		gasPrice = *tx.GasPriceWei // будет каститься в numeric
	}
	if tx.Status != nil {
		status = int16(*tx.Status)
	}

	q := `
INSERT INTO transactions(
  hash, chain_id, block_number, block_time,
  from_addr, to_addr,
  value_wei, nonce, tx_type, gas, gas_price_wei, status
) VALUES (
  $1, $2, $3, $4,
  $5, $6,
  $7::numeric, $8, $9, $10, $11::numeric, $12
)
ON CONFLICT(hash) DO UPDATE SET
  chain_id = EXCLUDED.chain_id,
  block_number = COALESCE(EXCLUDED.block_number, transactions.block_number),
  block_time   = COALESCE(EXCLUDED.block_time,   transactions.block_time),
  from_addr    = EXCLUDED.from_addr,
  to_addr      = COALESCE(EXCLUDED.to_addr, transactions.to_addr),
  value_wei    = EXCLUDED.value_wei,
  nonce        = EXCLUDED.nonce,
  tx_type      = EXCLUDED.tx_type,
  gas          = EXCLUDED.gas,
  gas_price_wei = COALESCE(EXCLUDED.gas_price_wei, transactions.gas_price_wei),
  status       = COALESCE(EXCLUDED.status, transactions.status),
  updated_at   = now()
`
	_, err := r.pool.Exec(cctx, q,
		tx.Hash, tx.ChainID, blockNum, blockTime,
		tx.FromAddr, toAddr,
		tx.ValueWei, int64(tx.Nonce), int(tx.TxType), int64(tx.Gas), gasPrice, status,
	)
	return err
}

func (r *Postgres) AddChatEvent(ctx context.Context, chatID int64, txHash string, eventType storage.TxEventType) error {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err := r.pool.Exec(cctx,
		`INSERT INTO chat_tx(chat_id, tx_hash, event_type) VALUES ($1, $2, $3)
		 ON CONFLICT DO NOTHING`,
		chatID, txHash, string(eventType),
	)
	return err
}

func (r *Postgres) ListHistory(ctx context.Context, chatID int64, limit int) ([]storage.HistoryItem, error) {
	if limit <= 0 {
		limit = 10
	}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	q := `
SELECT
  c.created_at,
  c.event_type,
  t.hash,
  t.block_number,
  t.block_time,
  t.from_addr,
  t.to_addr,
  t.value_wei::text,
  t.status
FROM chat_tx c
JOIN transactions t ON t.hash = c.tx_hash
WHERE c.chat_id = $1
ORDER BY c.created_at DESC
LIMIT $2
`
	rows, err := r.pool.Query(cctx, q, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []storage.HistoryItem
	for rows.Next() {
		var (
			at        time.Time
			etype     string
			hash      string
			blockNum  *int64
			blockTime *time.Time
			from      string
			to        *string
			valueWei  string
			status    *int16
		)

		if err := rows.Scan(&at, &etype, &hash, &blockNum, &blockTime, &from, &to, &valueWei, &status); err != nil {
			return nil, err
		}

		var bn *uint64
		if blockNum != nil {
			u := uint64(*blockNum)
			bn = &u
		}
		var st *uint8
		if status != nil {
			u := uint8(*status)
			st = &u
		}

		out = append(out, storage.HistoryItem{
			At: at, EventType: storage.TxEventType(etype),
			Hash: hash, BlockNum: bn, BlockTime: blockTime,
			FromAddr: from, ToAddr: to, ValueWei: valueWei, Status: st,
		})
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return out, nil
}

func (r *Postgres) String() string { return fmt.Sprintf("pgrepo(%p)", r.pool) }
