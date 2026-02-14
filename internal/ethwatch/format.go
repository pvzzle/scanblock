package ethwatch

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

var weiPerEth = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

func WeiToEthString(wei *big.Int) string {
	if wei == nil {
		return "0"
	}
	r := new(big.Rat).SetInt(wei)
	r.Quo(r, new(big.Rat).SetInt(weiPerEth))
	// 18 знаков слишком много для текста; обрежем до 6 после точки
	f, _ := r.Float64()
	return fmt.Sprintf("%.6f", f)
}

func FormatTxNotification(hash common.Hash, from common.Address, to *common.Address, valueWei *big.Int, blockNum uint64, blockTime uint64) string {
	toStr := "contract-creation"
	if to != nil {
		toStr = to.Hex()
	}
	tm := time.Unix(int64(blockTime), 0).UTC().Format(time.RFC3339)
	return fmt.Sprintf(
		"🔔 New tx\n\nHash: %s\nFrom: %s\nTo: %s\nValue: %s ETH\nBlock: #%d\nTime: %s",
		hash.Hex(),
		from.Hex(),
		toStr,
		WeiToEthString(valueWei),
		blockNum,
		tm,
	)
}
