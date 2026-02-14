package tg

import (
	"errors"
	"math/big"
	"regexp"
	"strings"
)

var (
	reTxHash  = regexp.MustCompile(`^(0x)?[0-9a-fA-F]{64}$`)
	reEthAddr = regexp.MustCompile(`^(0x)?[0-9a-fA-F]{40}$`)
	weiPerEth = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	ErrInvalidAmount = errors.New("invalid eth amount")
)

func IsTxHash(s string) bool {
	s = strings.TrimSpace(s)
	return reTxHash.MatchString(s)
}

func IsEthAddress(s string) bool {
	s = strings.TrimSpace(s)
	return reEthAddr.MatchString(s)
}

// ParseEthToWei парсит ETH-строку ("1.5", "0,5") в Wei (floor), требует > 0.
func ParseEthToWei(amount string) (*big.Int, error) {
	amount = strings.TrimSpace(amount)
	amount = strings.ReplaceAll(amount, ",", ".")

	r, ok := new(big.Rat).SetString(amount)
	if !ok || r.Sign() <= 0 {
		return nil, ErrInvalidAmount
	}

	r.Mul(r, new(big.Rat).SetInt(weiPerEth))

	// floor(r)
	out := new(big.Int)
	out.Div(r.Num(), r.Denom())
	if out.Sign() <= 0 {
		return nil, ErrInvalidAmount
	}

	return out, nil
}
