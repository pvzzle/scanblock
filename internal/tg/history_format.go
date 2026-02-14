package tg

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/pvzzle/scanblock/internal/ethwatch"
	"github.com/pvzzle/scanblock/internal/storage"
)

func FormatHistory(items []storage.HistoryItem) string {
	var sb strings.Builder
	sb.WriteString("🕘 History (последние 10)\n\n")

	for _, it := range items {
		valWei := new(big.Int)
		_, _ = valWei.SetString(it.ValueWei, 10)
		valEth := ethwatch.WeiToEthString(valWei)

		hashShort := shortenHash(it.Hash)

		status := ""
		if it.Status != nil {
			if *it.Status == 1 {
				status = " ✅"
			} else if *it.Status == 0 {
				status = " ❌"
			}
		}

		bn := ""
		if it.BlockNum != nil {
			bn = fmt.Sprintf(" #%d", *it.BlockNum)
		}

		sb.WriteString(fmt.Sprintf(
			"• %s (%s)%s\n  %s ETH%s\n",
			hashShort, it.EventType, bn, valEth, status,
		))
	}

	return sb.String()
}

func shortenHash(h string) string {
	if len(h) <= 14 {
		return h
	}
	return h[:10] + "…" + h[len(h)-4:]
}
