package tg

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/pvzzle/scanblock/internal/bus"
	"github.com/pvzzle/scanblock/internal/ethwatch"
	"github.com/pvzzle/scanblock/internal/storage"
	"github.com/pvzzle/scanblock/internal/subs"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	cbSearch    = "search"
	cbSubscribe = "subscribe"

	cbSubLarge  = "sub_large"
	cbSubWallet = "sub_wallet"

	cbMySubs      = "my_subs"
	cbUnsubLarge  = "unsub_large"
	cbUnsubWallet = "unsub_wallet"
	cbUnsubAll    = "unsub_all"
	cbBackToMain  = "back_main"

	cbHistory = "history"
)

type Service struct {
	bot     *tgbot.Bot
	eth     *ethclient.Client
	chainID *big.Int

	subStore *subs.Store
	notifyCh <-chan bus.Notification

	state *StateStore

	repo storage.Repository
}

func NewService(
	b *tgbot.Bot,
	eth *ethclient.Client,
	chainID *big.Int,
	subStore *subs.Store,
	notifyCh <-chan bus.Notification,
	repo storage.Repository,
) *Service {
	s := &Service{
		bot:      b,
		eth:      eth,
		chainID:  chainID,
		subStore: subStore,
		notifyCh: notifyCh,
		state:    NewStateStore(),
		repo:     repo,
	}
	s.registerHandlers()
	return s
}

func (s *Service) registerHandlers() {
	s.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/start", tgbot.MatchTypeExact, s.onStart)

	s.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, cbSearch, tgbot.MatchTypeExact, s.onCbSearch)
	s.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, cbSubscribe, tgbot.MatchTypeExact, s.onCbSubscribe)
	s.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, cbSubLarge, tgbot.MatchTypeExact, s.onCbSubLarge)
	s.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, cbSubWallet, tgbot.MatchTypeExact, s.onCbSubWallet)

	s.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, cbMySubs, tgbot.MatchTypeExact, s.onCbMySubs)
	s.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, cbUnsubLarge, tgbot.MatchTypeExact, s.onCbUnsubLarge)
	s.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, cbUnsubWallet, tgbot.MatchTypeExact, s.onCbUnsubWallet)
	s.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, cbUnsubAll, tgbot.MatchTypeExact, s.onCbUnsubAll)
	s.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, cbBackToMain, tgbot.MatchTypeExact, s.onCbBackToMain)

	s.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "", tgbot.MatchTypePrefix, s.onAnyText)
	s.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, cbHistory, tgbot.MatchTypeExact, s.onCbHistory)

}

func (s *Service) StartNotifyLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case n := <-s.notifyCh:
			_, err := s.bot.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: n.ChatID,
				Text:   n.Text,
			})
			if err != nil {
				log.Printf("[tg] send notify error: %v", err)
			}
		}
	}
}

func (s *Service) onStart(ctx context.Context, b *tgbot.Bot, upd *models.Update) {
	if upd.Message == nil {
		return
	}
	chatID := upd.Message.Chat.ID
	s.state.Set(chatID, StateIdle)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Привет! Я могу искать транзакции и управлять подписками.\n\nВыбери действие:",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "Search", CallbackData: cbSearch},
					{Text: "Subscribe", CallbackData: cbSubscribe},
				},
				{
					{Text: "My subscriptions", CallbackData: cbMySubs},
					{Text: "History", CallbackData: cbHistory},
				},
			},
		},
	})
}

func (s *Service) onCbSearch(ctx context.Context, b *tgbot.Bot, upd *models.Update) {
	cb := upd.CallbackQuery
	if cb == nil || cb.Message.Type == models.MaybeInaccessibleMessageTypeInaccessibleMessage {
		return
	}
	_ = s.answerCallback(ctx, b, cb.ID)

	chatID := cb.Message.Message.Chat.ID
	s.state.Set(chatID, StateAwaitTxHash)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Введи хэш транзакции (0x...):",
	})
}

func (s *Service) onCbSubscribe(ctx context.Context, b *tgbot.Bot, upd *models.Update) {
	cb := upd.CallbackQuery
	if cb == nil || cb.Message.Type == models.MaybeInaccessibleMessageTypeInaccessibleMessage {
		return
	}
	_ = s.answerCallback(ctx, b, cb.ID)

	chatID := cb.Message.Message.Chat.ID
	s.state.Set(chatID, StateIdle)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Что отслеживать?",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{{Text: "Крупные объемы (ETH)", CallbackData: cbSubLarge}},
				{{Text: "Кошелёк (sender/receiver)", CallbackData: cbSubWallet}},
			},
		},
	})
}

func (s *Service) onCbSubLarge(ctx context.Context, b *tgbot.Bot, upd *models.Update) {
	cb := upd.CallbackQuery
	if cb == nil || cb.Message.Type == models.MaybeInaccessibleMessageTypeInaccessibleMessage {
		return
	}
	_ = s.answerCallback(ctx, b, cb.ID)

	chatID := cb.Message.Message.Chat.ID
	s.state.Set(chatID, StateAwaitLargeAmountEth)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Введи сумму в ETH (> 0), например: 1.5",
	})
}

func (s *Service) onCbSubWallet(ctx context.Context, b *tgbot.Bot, upd *models.Update) {
	cb := upd.CallbackQuery
	if cb == nil || cb.Message.Type == models.MaybeInaccessibleMessageTypeInaccessibleMessage {
		return
	}
	_ = s.answerCallback(ctx, b, cb.ID)

	chatID := cb.Message.Message.Chat.ID
	s.state.Set(chatID, StateAwaitWalletAddress)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Введи адрес кошелька (0x...):",
	})
}

func (s *Service) onAnyText(ctx context.Context, b *tgbot.Bot, upd *models.Update) {
	if upd.Message == nil {
		return
	}
	chatID := upd.Message.Chat.ID
	text := strings.TrimSpace(upd.Message.Text)

	// команды — не обрабатываем тут
	if strings.HasPrefix(text, "/") {
		return
	}

	switch s.state.Get(chatID) {
	case StateAwaitTxHash:
		s.state.Set(chatID, StateIdle)
		s.handleSearchTx(ctx, b, chatID, text)

	case StateAwaitLargeAmountEth:
		s.handleSetLarge(ctx, b, chatID, text)

	case StateAwaitWalletAddress:
		s.handleSetWallet(ctx, b, chatID, text)

	default:
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Используй /start, чтобы открыть меню.",
		})
	}
}

func (s *Service) handleSearchTx(ctx context.Context, b *tgbot.Bot, chatID int64, hashStr string) {
	if !IsTxHash(hashStr) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Похоже, это не хэш транзакции. Ожидаю 0x + 64 hex символа.",
		})
		return
	}

	h := common.HexToHash(hashStr)

	tx, isPending, err := s.eth.TransactionByHash(ctx, h)
	if err != nil {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("Не нашёл транзакцию: %v", err),
		})
		return
	}

	signer := types.LatestSignerForChainID(s.chainID)
	from, _ := types.Sender(signer, tx)

	to := tx.To()
	toStr := "contract-creation"
	if to != nil {
		toStr = to.Hex()
	}

	var gasPriceWei *string
	if gp := tx.GasPrice(); gp != nil {
		s := gp.String()
		gasPriceWei = &s
	}

	var (
		blockNum  *uint64
		blockTime *time.Time
		status    *uint8
	)

	if !isPending {
		receipt, rerr := s.eth.TransactionReceipt(ctx, h)
		if rerr == nil && receipt != nil {
			bn := receipt.BlockNumber.Uint64()
			blockNum = &bn

			st := uint8(receipt.Status) // 1/0
			status = &st

			block, berr := s.eth.BlockByNumber(ctx, receipt.BlockNumber)
			if berr == nil && block != nil {
				tm := time.Unix(int64(block.Time()), 0).UTC()
				blockTime = &tm
			}
		}
	}

	txRec := storage.TxRecord{
		Hash:        tx.Hash().Hex(),
		ChainID:     s.chainID.String(),
		BlockNum:    blockNum,
		BlockTime:   blockTime,
		FromAddr:    from.Hex(),
		ToAddr:      &toStr,
		ValueWei:    tx.Value().String(),
		Nonce:       tx.Nonce(),
		TxType:      tx.Type(),
		Gas:         tx.Gas(),
		GasPriceWei: gasPriceWei,
		Status:      status,
	}

	if err := s.repo.UpsertTx(ctx, txRec); err != nil {
		log.Printf("[tg] db upsert search tx error: %v", err)
	}
	_ = s.repo.AddChatEvent(ctx, chatID, txRec.Hash, storage.EventSearch)

	valueEth := ethwatch.WeiToEthString(tx.Value())

	msg := fmt.Sprintf(
		"✅ Транзакция найдена\n\nHash: %s\nFrom: %s\nTo: %s\nValue: %s ETH\nNonce: %d\nType: %d\nPending: %v\nGas: %d",
		tx.Hash().Hex(),
		from.Hex(),
		toStr,
		valueEth,
		tx.Nonce(),
		tx.Type(),
		isPending,
		tx.Gas(),
	)

	// Если уже в блоке — добавим статус/блок/время
	if !isPending {
		receipt, rerr := s.eth.TransactionReceipt(ctx, h)
		if rerr == nil && receipt != nil {
			status := "FAILED"
			if receipt.Status == 1 {
				status = "SUCCESS"
			}

			block, berr := s.eth.BlockByNumber(ctx, receipt.BlockNumber)
			var tm string
			if berr == nil && block != nil {
				tm = time.Unix(int64(block.Time()), 0).UTC().Format(time.RFC3339)
			}

			msg += fmt.Sprintf("\nStatus: %s\nBlock: #%s\nTime: %s\nGasUsed: %d",
				status,
				receipt.BlockNumber.String(),
				tm,
				receipt.GasUsed,
			)
		}
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   msg,
	})
}

func (s *Service) handleSetLarge(ctx context.Context, b *tgbot.Bot, chatID int64, amountStr string) {
	amountStr = strings.ReplaceAll(amountStr, ",", ".")
	f, ok := new(big.Rat).SetString(amountStr)
	if !ok || f.Sign() <= 0 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Нужно число > 0 (например 0.5 или 10). Попробуй ещё раз.",
		})
		return
	}

	weiPerEth := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	f.Mul(f, weiPerEth)

	minWei, err := ParseEthToWei(amountStr)
	if err != nil {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Нужно число > 0 (например 0.5 или 10). Попробуй ещё раз.",
		})
		return
	}

	s.subStore.SetLargeTxMin(chatID, minWei)
	s.state.Set(chatID, StateIdle)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("✅ Ок! Буду уведомлять о транзакциях с Value >= %s ETH.", amountStr),
	})
}

func (s *Service) handleSetWallet(ctx context.Context, b *tgbot.Bot, chatID int64, addrStr string) {
	if !IsEthAddress(addrStr) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Похоже, это не адрес. Ожидаю 0x + 40 hex символов.",
		})
		return
	}
	addr := common.HexToAddress(addrStr)

	s.subStore.SetWallet(chatID, addr)
	s.state.Set(chatID, StateIdle)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("✅ Ок! Буду уведомлять о транзакциях, где участвует %s.", addr.Hex()),
	})
}

func (s *Service) answerCallback(ctx context.Context, b *tgbot.Bot, callbackID string) error {
	_, err := b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
	})
	return err
}

func (s *Service) onCbMySubs(ctx context.Context, b *tgbot.Bot, upd *models.Update) {
	cb := upd.CallbackQuery
	if cb == nil || cb.Message.Type == models.MaybeInaccessibleMessageTypeInaccessibleMessage {
		return
	}
	_ = s.answerCallback(ctx, b, cb.ID)

	chatID := cb.Message.Message.Chat.ID
	s.state.Set(chatID, StateIdle)

	s.sendMySubs(ctx, b, chatID)
}

func (s *Service) onCbUnsubLarge(ctx context.Context, b *tgbot.Bot, upd *models.Update) {
	cb := upd.CallbackQuery
	if cb == nil || cb.Message.Type == models.MaybeInaccessibleMessageTypeInaccessibleMessage {
		return
	}
	_ = s.answerCallback(ctx, b, cb.ID)

	chatID := cb.Message.Message.Chat.ID
	s.subStore.ClearLargeTx(chatID)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "✅ Подписка на крупные объемы удалена.",
	})
	s.sendMySubs(ctx, b, chatID)
}

func (s *Service) onCbUnsubWallet(ctx context.Context, b *tgbot.Bot, upd *models.Update) {
	cb := upd.CallbackQuery
	if cb == nil || cb.Message.Type == models.MaybeInaccessibleMessageTypeInaccessibleMessage {
		return
	}
	_ = s.answerCallback(ctx, b, cb.ID)

	chatID := cb.Message.Message.Chat.ID
	s.subStore.ClearWallet(chatID)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "✅ Подписка на кошелёк удалена.",
	})
	s.sendMySubs(ctx, b, chatID)
}

func (s *Service) onCbUnsubAll(ctx context.Context, b *tgbot.Bot, upd *models.Update) {
	cb := upd.CallbackQuery
	if cb == nil || cb.Message.Type == models.MaybeInaccessibleMessageTypeInaccessibleMessage {
		return
	}
	_ = s.answerCallback(ctx, b, cb.ID)

	chatID := cb.Message.Message.Chat.ID
	s.subStore.ClearAll(chatID)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "✅ Все подписки удалены.",
	})
	s.sendMySubs(ctx, b, chatID)
}

func (s *Service) onCbBackToMain(ctx context.Context, b *tgbot.Bot, upd *models.Update) {
	cb := upd.CallbackQuery
	if cb == nil || cb.Message.Type == models.MaybeInaccessibleMessageTypeInaccessibleMessage {
		return
	}
	_ = s.answerCallback(ctx, b, cb.ID)

	chatID := cb.Message.Message.Chat.ID
	s.state.Set(chatID, StateIdle)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Главное меню:",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "Search", CallbackData: cbSearch},
					{Text: "Subscribe", CallbackData: cbSubscribe},
				},
				{
					{Text: "My subscriptions", CallbackData: cbMySubs},
					{Text: "History", CallbackData: cbHistory},
				},
			},
		},
	})
}

func (s *Service) sendMySubs(ctx context.Context, b *tgbot.Bot, chatID int64) {
	u, ok := s.subStore.GetCopy(chatID)

	var lines []string
	lines = append(lines, "📌 Твои подписки:")

	if !ok || (u.LargeTxMinWei == nil && u.Wallet == nil) {
		lines = append(lines, "— нет активных подписок")
	} else {
		if u.LargeTxMinWei != nil {
			lines = append(lines, fmt.Sprintf("— Крупные объемы: Value >= %s ETH", ethwatch.WeiToEthString(u.LargeTxMinWei)))
		} else {
			lines = append(lines, "— Крупные объемы: (нет)")
		}
		if u.Wallet != nil {
			lines = append(lines, fmt.Sprintf("— Кошелёк: %s", u.Wallet.Hex()))
		} else {
			lines = append(lines, "— Кошелёк: (нет)")
		}
	}

	// кнопки удаления показываем всегда (удобнее)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   strings.Join(lines, "\n"),
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{{Text: "Удалить: крупные объемы", CallbackData: cbUnsubLarge}},
				{{Text: "Удалить: кошелёк", CallbackData: cbUnsubWallet}},
				{{Text: "Удалить всё", CallbackData: cbUnsubAll}},
				{{Text: "Назад", CallbackData: cbBackToMain}},
			},
		},
	})
}

func (s *Service) onCbHistory(ctx context.Context, b *tgbot.Bot, upd *models.Update) {
	cb := upd.CallbackQuery
	if cb == nil || cb.Message.Type == models.MaybeInaccessibleMessageTypeInaccessibleMessage {
		return
	}
	_ = s.answerCallback(ctx, b, cb.ID)

	chatID := cb.Message.Message.Chat.ID

	items, err := s.repo.ListHistory(ctx, chatID, 10)
	if err != nil {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("Ошибка чтения истории: %v", err),
		})
		return
	}

	if len(items) == 0 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "История пуста.",
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{
					{{Text: "Назад", CallbackData: cbBackToMain}},
				},
			},
		})
		return
	}

	text := FormatHistory(items)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{{Text: "Назад", CallbackData: cbBackToMain}},
			},
		},
	})
}
