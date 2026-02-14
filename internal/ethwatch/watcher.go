package ethwatch

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/pvzzle/scanblock/internal/bus"
	"github.com/pvzzle/scanblock/internal/storage"
	"github.com/pvzzle/scanblock/internal/subs"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type WatcherConfig struct {
	Workers     int
	TasksBuffer int
}

type TxTask struct {
	Tx        *types.Transaction
	BlockNum  uint64
	BlockTime uint64
}

type Watcher struct {
	client  *ethclient.Client
	chainID *big.Int

	subStore *subs.Store
	notifyCh chan<- bus.Notification

	cfg WatcherConfig

	tasks chan TxTask
	wg    sync.WaitGroup

	repo storage.Repository
}

func NewWatcher(
	client *ethclient.Client,
	chainID *big.Int,
	subStore *subs.Store,
	notifyCh chan<- bus.Notification,
	repo storage.Repository,
	cfg WatcherConfig,
) *Watcher {

	if cfg.Workers <= 0 {
		cfg.Workers = 8
	}

	if cfg.TasksBuffer <= 0 {
		cfg.TasksBuffer = 1024
	}

	return &Watcher{
		client:   client,
		chainID:  chainID,
		subStore: subStore,
		notifyCh: notifyCh,
		cfg:      cfg,
		tasks:    make(chan TxTask, cfg.TasksBuffer),
		repo:     repo,
	}
}

func (w *Watcher) Start(ctx context.Context) error {
	w.startWorkers(ctx)
	defer w.stopWorkers()

	headers := make(chan *types.Header, 128)

	sub, err := w.client.SubscribeNewHead(ctx, headers)
	if err != nil {
		return fmt.Errorf("SubscribeNewHead: %w", err)
	}
	defer sub.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-sub.Err():
			return fmt.Errorf("subscription error: %w", err)

		case h := <-headers:
			if h == nil {
				continue
			}

			block, err := w.client.BlockByHash(ctx, h.Hash())
			if err != nil {
				log.Printf("[WATCHER] block fetch error: %v", err)
				continue
			}

			for _, tx := range block.Transactions() {
				task := TxTask{
					Tx:        tx,
					BlockNum:  block.NumberU64(),
					BlockTime: block.Time(),
				}

				select {
				case w.tasks <- task:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}
}

func (w *Watcher) startWorkers(ctx context.Context) {
	signer := types.LatestSignerForChainID(w.chainID)

	for i := 0; i < w.cfg.Workers; i++ {
		w.wg.Add(1)
		go func(workerID int) {
			defer w.wg.Done()

			for {
				select {
				case <-ctx.Done():
					return

				case task, ok := <-w.tasks:
					if !ok {
						return
					}
					w.handleTask(ctx, signer, task)
				}
			}
		}(i)
	}
}

func (w *Watcher) stopWorkers() {
	close(w.tasks)
	w.wg.Wait()
}

func (w *Watcher) handleTask(ctx context.Context, signer types.Signer, task TxTask) {
	tx := task.Tx

	from, err := types.Sender(signer, tx)
	if err != nil {
		// Во избежание так называемых legacy edge cases.
		return
	}

	var to *common.Address = tx.To()
	val := tx.Value()
	if val == nil {
		val = big.NewInt(0)
	}

	recipients := w.subStore.MatchTx(from, to, val)
	if len(recipients) == 0 {
		return
	}

	// 1) сохраняем саму транзакцию
	var toStr *string
	if to != nil {
		x := to.Hex()
		toStr = &x
	}
	var bt = time.Unix(int64(task.BlockTime), 0).UTC()
	blockTime := &bt
	blockNum := &task.BlockNum

	var gasPriceWei *string
	if gp := tx.GasPrice(); gp != nil {
		s := gp.String()
		gasPriceWei = &s
	}

	txRec := storage.TxRecord{
		Hash:        tx.Hash().Hex(),
		ChainID:     w.chainID.String(),
		BlockNum:    blockNum,
		BlockTime:   blockTime,
		FromAddr:    from.Hex(),
		ToAddr:      toStr,
		ValueWei:    val.String(),
		Nonce:       tx.Nonce(),
		TxType:      tx.Type(),
		Gas:         tx.Gas(),
		GasPriceWei: gasPriceWei,
		// Status неизвестен без receipt (можно расширить при желании)
		Status: nil,
	}

	if err := w.repo.UpsertTx(ctx, txRec); err != nil {
		log.Printf("[watcher] db upsert tx error: %v", err)
		// не возвращаем — уведомления важнее
	}

	// 2) отправляем уведомления + пишем событие в историю каждому чату
	text := FormatTxNotification(tx.Hash(), from, to, val, task.BlockNum, task.BlockTime)

	for _, chatID := range recipients {
		_ = w.repo.AddChatEvent(ctx, chatID, txRec.Hash, storage.EventNotify)

		select {
		case w.notifyCh <- bus.Notification{ChatID: chatID, Text: text}:
		case <-ctx.Done():
			return
		}
	}
}
