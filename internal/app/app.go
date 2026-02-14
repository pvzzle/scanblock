package app

import (
	"context"
	"fmt"
	"log"

	"github.com/pvzzle/scanblock/internal/bus"
	"github.com/pvzzle/scanblock/internal/ethwatch"
	"github.com/pvzzle/scanblock/internal/storage/pg"
	"github.com/pvzzle/scanblock/internal/subs"
	"github.com/pvzzle/scanblock/internal/tg"

	"github.com/ethereum/go-ethereum/ethclient"
	tgbot "github.com/go-telegram/bot"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Run(ctx context.Context) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	pgPool, err := pgxpool.New(ctx, cfg.PostgresURL)
	if err != nil {
		return fmt.Errorf("pgxpool new: %w", err)
	}
	defer pgPool.Close()

	repo := pg.New(pgPool)
	if err := repo.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}

	ethCl, err := ethclient.DialContext(ctx, cfg.EthWSURL)
	if err != nil {
		return fmt.Errorf("dial eth ws: %w", err)
	}

	chainID, err := ethCl.NetworkID(ctx)
	if err != nil {
		return fmt.Errorf("network id: %w", err)
	}

	subStore := subs.NewStore()

	notifyCh := make(chan bus.Notification, cfg.NotifyBuffer)

	b, err := tgbot.New(cfg.TelegramToken,
		tgbot.WithDebug(),
		tgbot.WithWorkers(4),
		tgbot.WithNotAsyncHandlers(),
	)
	if err != nil {
		return fmt.Errorf("telegram bot init: %w", err)
	}

	tgSvc := tg.NewService(b, ethCl, chainID, subStore, notifyCh, repo)
	watcher := ethwatch.NewWatcher(ethCl, chainID, subStore, notifyCh, repo, ethwatch.WatcherConfig{
		Workers:     cfg.WatcherWorkers,
		TasksBuffer: cfg.TasksBuffer,
	})

	go func() {
		if err := watcher.Start(ctx); err != nil {
			log.Printf("[WATCHER] stopped: %v", err)
		}
	}()

	go tgSvc.StartNotifyLoop(ctx)

	log.Printf("started. chain_id=%s workers=%d", chainID.String(), cfg.WatcherWorkers)
	b.Start(ctx)

	return nil
}
