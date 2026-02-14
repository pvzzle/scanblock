package subs

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestStore_MatchTx_LargeVolume(t *testing.T) {
	s := NewStore()
	chatID := int64(42)

	oneEth := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	s.SetLargeTxMin(chatID, oneEth)

	from := common.HexToAddress("0x1111111111111111111111111111111111111111")
	to := common.HexToAddress("0x2222222222222222222222222222222222222222")

	twoEth := new(big.Int).Mul(oneEth, big.NewInt(2))
	got := s.MatchTx(from, &to, twoEth)
	if len(got) != 1 || got[0] != chatID {
		t.Fatalf("expected match for 2 ETH, got=%v", got)
	}

	halfEth := new(big.Int).Div(oneEth, big.NewInt(2))
	got = s.MatchTx(from, &to, halfEth)
	if len(got) != 0 {
		t.Fatalf("expected no match for 0.5 ETH, got=%v", got)
	}
}

func TestStore_MatchTx_Wallet(t *testing.T) {
	s := NewStore()
	chatID := int64(7)

	wallet := common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	s.SetWallet(chatID, wallet)

	other := common.HexToAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	got := s.MatchTx(wallet, &other, big.NewInt(1))
	if len(got) != 1 || got[0] != chatID {
		t.Fatalf("expected match by sender, got=%v", got)
	}

	got = s.MatchTx(other, &wallet, big.NewInt(1))
	if len(got) != 1 || got[0] != chatID {
		t.Fatalf("expected match by receiver, got=%v", got)
	}
}

func TestStore_ClearAndCleanup(t *testing.T) {
	s := NewStore()
	chatID := int64(1)

	oneEth := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	addr := common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	s.SetLargeTxMin(chatID, oneEth)
	s.SetWallet(chatID, addr)

	s.ClearLargeTx(chatID)
	u, ok := s.GetCopy(chatID)
	if !ok || u.Wallet == nil || u.LargeTxMinWei != nil {
		t.Fatalf("expected only wallet to remain, ok=%v, subs=%+v", ok, u)
	}

	s.ClearWallet(chatID)
	_, ok = s.GetCopy(chatID)
	if ok {
		t.Fatalf("expected cleanup (no subs) => no record")
	}
}

func TestStore_GetCopy_IsCopy(t *testing.T) {
	s := NewStore()
	chatID := int64(1)

	oneEth := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	s.SetLargeTxMin(chatID, oneEth)

	u, ok := s.GetCopy(chatID)
	if !ok || u.LargeTxMinWei == nil {
		t.Fatalf("expected copy")
	}

	u.LargeTxMinWei.SetInt64(0)

	u2, ok := s.GetCopy(chatID)
	if !ok || u2.LargeTxMinWei == nil || u2.LargeTxMinWei.Cmp(oneEth) != 0 {
		t.Fatalf("expected stored value unchanged, got=%v", u2.LargeTxMinWei)
	}
}
