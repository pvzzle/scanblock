package ethwatch

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestWeiToEthString(t *testing.T) {
	oneEth := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	if got := WeiToEthString(oneEth); got != "1.000000" {
		t.Fatalf("expected 1.000000, got %q", got)
	}

	half := new(big.Int).Div(oneEth, big.NewInt(2))
	if got := WeiToEthString(half); got != "0.500000" {
		t.Fatalf("expected 0.500000, got %q", got)
	}
}

func TestFormatTxNotification(t *testing.T) {
	hash := common.HexToHash("0x" + strings.Repeat("11", 32))
	from := common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	to := common.HexToAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	oneEth := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	txt := FormatTxNotification(hash, from, &to, oneEth, 123, 1700000000)
	if txt == "" {
		t.Fatal("expected non-empty")
	}
	if !contains(txt, hash.Hex()) {
		t.Fatalf("expected hash in text: %s", txt)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (func() bool { return (stringIndex(s, sub) >= 0) })())
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
