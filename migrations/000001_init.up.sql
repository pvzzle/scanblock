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

  status SMALLINT NULL, -- 1 успешный успех, 0 неуспешный успех

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