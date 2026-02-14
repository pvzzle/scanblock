package ethwatch

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/pvzzle/scanblock/internal/bus"
	"github.com/pvzzle/scanblock/internal/storage"
	"github.com/pvzzle/scanblock/internal/subs"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type mockRepo struct {
	mu      sync.Mutex
	upserts []storage.TxRecord
	events  []struct {
		chatID int64
		hash   string
		etype  storage.TxEventType
	}
}

func (m *mockRepo) EnsureSchema(ctx context.Context) error { return nil }
func (m *mockRepo) UpsertTx(ctx context.Context, tx storage.TxRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upserts = append(m.upserts, tx)
	return nil
}
func (m *mockRepo) AddChatEvent(ctx context.Context, chatID int64, txHash string, eventType storage.TxEventType) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, struct {
		chatID int64
		hash   string
		etype  storage.TxEventType
	}{chatID: chatID, hash: txHash, etype: eventType})
	return nil
}
func (m *mockRepo) ListHistory(ctx context.Context, chatID int64, limit int) ([]storage.HistoryItem, error) {
	return nil, nil
}

func TestWatcher_handleTask_PersistsAndNotifies(t *testing.T) {
	ctx := context.Background()

	chainID := big.NewInt(1)
	signer := types.LatestSignerForChainID(chainID)

	// key -> from address
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	from := crypto.PubkeyToAddress(key.PublicKey)

	to := common.HexToAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	oneEth := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	unsigned := types.NewTx(&types.LegacyTx{
		Nonce:    0,
		To:       &to,
		Value:    oneEth,
		Gas:      21000,
		GasPrice: big.NewInt(1),
	})
	tx, err := types.SignTx(unsigned, signer, key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	subStore := subs.NewStore()
	chatID := int64(99)

	subStore.SetWallet(chatID, from)

	notifyCh := make(chan bus.Notification, 1)
	repo := &mockRepo{}

	w := &Watcher{
		client:   nil, // not used in handleTask
		chainID:  chainID,
		subStore: subStore,
		notifyCh: notifyCh,
		repo:     repo,
	}

	task := TxTask{
		Tx:        tx,
		BlockNum:  123,
		BlockTime: uint64(time.Now().Unix()),
	}

	w.handleTask(ctx, signer, task)

	select {
	case n := <-notifyCh:
		if n.ChatID != chatID {
			t.Fatalf("expected chatID=%d, got=%d", chatID, n.ChatID)
		}
		if n.Text == "" {
			t.Fatal("expected non-empty notification text")
		}
	default:
		t.Fatal("expected notification")
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()

	if len(repo.upserts) != 1 {
		t.Fatalf("expected 1 upsert, got=%d", len(repo.upserts))
	}
	if repo.upserts[0].Hash != tx.Hash().Hex() {
		t.Fatalf("expected upsert hash=%s got=%s", tx.Hash().Hex(), repo.upserts[0].Hash)
	}

	if len(repo.events) != 1 {
		t.Fatalf("expected 1 event, got=%d", len(repo.events))
	}
	if repo.events[0].chatID != chatID || repo.events[0].etype != storage.EventNotify {
		t.Fatalf("unexpected event: %+v", repo.events[0])
	}
}
