package tg

import (
	"testing"
	"time"

	"github.com/pvzzle/scanblock/internal/storage"
)

func TestFormatHistory(t *testing.T) {
	now := time.Date(2026, 2, 14, 10, 0, 0, 0, time.UTC)
	bn := uint64(123)
	st := uint8(1)

	items := []storage.HistoryItem{
		{
			At:        now,
			EventType: storage.EventNotify,
			Hash:      "0x" + repeat("1", 64),
			BlockNum:  &bn,
			ValueWei:  "1000000000000000000", // 1 ETH
			Status:    &st,
		},
	}

	txt := FormatHistory(items)

	if txt == "" {
		t.Fatal("expected non-empty")
	}
	if !has(txt, "…") {
		t.Fatalf("expected shortened hash: %s", txt)
	}
	if !has(txt, "1.000000 ETH") {
		t.Fatalf("expected eth value: %s", txt)
	}
	if !has(txt, "notify") {
		t.Fatalf("expected event type: %s", txt)
	}
	if !has(txt, "#123") {
		t.Fatalf("expected block num: %s", txt)
	}
	if !has(txt, "✅") {
		t.Fatalf("expected status: %s", txt)
	}
}

func has(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
