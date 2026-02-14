package tg

import "sync"

type ChatState int

const (
	StateIdle ChatState = iota
	StateAwaitTxHash
	StateAwaitLargeAmountEth
	StateAwaitWalletAddress
)

type StateStore struct {
	mu    sync.Mutex
	state map[int64]ChatState
}

func NewStateStore() *StateStore {
	return &StateStore{state: make(map[int64]ChatState)}
}

func (s *StateStore) Set(chatID int64, st ChatState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state[chatID] = st
}

func (s *StateStore) Get(chatID int64) ChatState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state[chatID]
}
