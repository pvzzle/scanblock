package tg

import (
	"math/big"
	"testing"
)

func TestParseEthToWei(t *testing.T) {
	oneEth := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	got, err := ParseEthToWei("1")
	if err != nil || got.Cmp(oneEth) != 0 {
		t.Fatalf("expected 1 ETH, got=%v err=%v", got, err)
	}

	half, err := ParseEthToWei("0.5")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	expHalf := new(big.Int).Div(oneEth, big.NewInt(2))
	if half.Cmp(expHalf) != 0 {
		t.Fatalf("expected 0.5 ETH in wei, got=%v", half)
	}

	_, err = ParseEthToWei("0")
	if err == nil {
		t.Fatalf("expected error for 0")
	}

	_, err = ParseEthToWei("-1")
	if err == nil {
		t.Fatalf("expected error for negative")
	}

	_, err = ParseEthToWei("abc")
	if err == nil {
		t.Fatalf("expected error for non-number")
	}

	got, err = ParseEthToWei("1,5")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	exp := new(big.Int).Mul(oneEth, big.NewInt(3))
	exp.Div(exp, big.NewInt(2))
	if got.Cmp(exp) != 0 {
		t.Fatalf("expected 1.5 ETH, got=%v", got)
	}
}

func TestValidators(t *testing.T) {
	if !IsTxHash("0x" + repeat("a", 64)) {
		t.Fatalf("expected valid tx hash")
	}
	if IsTxHash("0x123") {
		t.Fatalf("expected invalid tx hash")
	}

	if !IsEthAddress("0x" + repeat("b", 40)) {
		t.Fatalf("expected valid address")
	}
	if IsEthAddress("0x" + repeat("b", 39)) {
		t.Fatalf("expected invalid address")
	}
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
