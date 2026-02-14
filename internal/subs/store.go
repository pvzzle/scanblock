package subs

import (
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

type UserSubs struct {
	LargeTxMinWei *big.Int
	Wallet        *common.Address
}

type Store struct {
	mu   sync.RWMutex
	data map[int64]*UserSubs
}

func NewStore() *Store {
	return &Store{data: make(map[int64]*UserSubs)}
}

func (s *Store) SetLargeTxMin(chatID int64, minWei *big.Int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u := s.getOrCreate(chatID)
	if minWei == nil {
		u.LargeTxMinWei = nil
		s.cleanupIfEmpty(chatID, u)
		return
	}
	u.LargeTxMinWei = new(big.Int).Set(minWei)
}

func (s *Store) SetWallet(chatID int64, addr common.Address) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u := s.getOrCreate(chatID)
	u.Wallet = &addr
}

func (s *Store) ClearLargeTx(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u := s.data[chatID]
	if u == nil {
		return
	}
	u.LargeTxMinWei = nil
	s.cleanupIfEmpty(chatID, u)
}

func (s *Store) ClearWallet(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u := s.data[chatID]
	if u == nil {
		return
	}
	u.Wallet = nil
	s.cleanupIfEmpty(chatID, u)
}

func (s *Store) ClearAll(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, chatID)
}

// GetCopy возвращает копию подписок пользователя (чтобы снаружи не было гонок/мутирования)
func (s *Store) GetCopy(chatID int64) (UserSubs, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u := s.data[chatID]
	if u == nil {
		return UserSubs{}, false
	}

	var out UserSubs
	if u.LargeTxMinWei != nil {
		out.LargeTxMinWei = new(big.Int).Set(u.LargeTxMinWei)
	}
	if u.Wallet != nil {
		a := *u.Wallet
		out.Wallet = &a
	}
	return out, true
}

func (s *Store) MatchTx(sender common.Address, receiver *common.Address, valueWei *big.Int) []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []int64
	for chatID, u := range s.data {
		if u == nil {
			continue
		}

		// large volume
		if u.LargeTxMinWei != nil && valueWei != nil && valueWei.Sign() > 0 {
			if valueWei.Cmp(u.LargeTxMinWei) >= 0 {
				out = append(out, chatID)
				continue
			}
		}

		// wallet
		if u.Wallet != nil {
			if sender == *u.Wallet {
				out = append(out, chatID)
				continue
			}
			if receiver != nil && *receiver == *u.Wallet {
				out = append(out, chatID)
				continue
			}
		}
	}
	return out
}

func (s *Store) getOrCreate(chatID int64) *UserSubs {
	u := s.data[chatID]
	if u == nil {
		u = &UserSubs{}
		s.data[chatID] = u
	}
	return u
}

func (s *Store) cleanupIfEmpty(chatID int64, u *UserSubs) {
	if u == nil {
		return
	}
	if u.LargeTxMinWei == nil && u.Wallet == nil {
		delete(s.data, chatID)
	}
}
