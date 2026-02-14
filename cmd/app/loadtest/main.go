package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pvzzle/scanblock/internal/storage"
	"github.com/pvzzle/scanblock/internal/storage/pg" // твой пакет repo (переименуй импорт под себя)

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"
)

type opType int

const (
	opWrite opType = iota
	opRead
)

type latencySample struct {
	d time.Duration
}

func main() {
	var (
		dsn       = flag.String("dsn", "", "Postgres DSN")
		dur       = flag.Duration("dur", 60*time.Second, "test duration")
		warmup    = flag.Duration("warmup", 5*time.Second, "warmup duration (not counted)")
		avgRPS    = flag.Int("avg-rps", 300, "avg RPS")
		peakRPS   = flag.Int("peak-rps", 1500, "peak RPS (during ramp)")
		ramp      = flag.Duration("ramp", 10*time.Second, "ramp-up duration to peak")
		rwRatio   = flag.Int("rw", 15, "R/W ratio, reads per 1 write (e.g. 15)")
		workers   = flag.Int("workers", 64, "concurrent workers")
		histLimit = flag.Int("hist-limit", 10, "history limit")
	)
	flag.Parse()

	if *dsn == "" {
		panic("dsn required")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	repo := pg.New(pool)
	if err := repo.EnsureSchema(ctx); err != nil {
		panic(err)
	}

	fmt.Println("starting warmup:", *warmup)
	runPhase(ctx, repo, *workers, *avgRPS, *avgRPS, 0, *warmup, *rwRatio, *histLimit, false)

	fmt.Println("starting measured test:", *dur)
	res := runPhase(ctx, repo, *workers, *avgRPS, *peakRPS, *ramp, *dur, *rwRatio, *histLimit, true)

	printReport(res)
}

type results struct {
	totalOps   uint64
	readOps    uint64
	writeOps   uint64
	errOps     uint64
	latencies  []time.Duration // measured ops only
	startedAt  time.Time
	finishedAt time.Time
}

func runPhase(
	ctx context.Context,
	repo storage.Repository,
	workers int,
	avgRPS int,
	peakRPS int,
	ramp time.Duration,
	dur time.Duration,
	rw int,
	histLimit int,
	collect bool,
) results {
	ctx, cancel := context.WithTimeout(ctx, dur)
	defer cancel()

	// RPS limiter with optional ramp to peak:
	// If ramp == 0 => constant avgRPS
	lim := rate.NewLimiter(rate.Limit(avgRPS), avgRPS)

	type job struct {
		op opType
	}

	jobs := make(chan job, 1024)

	var (
		res results
		mu  sync.Mutex
	)

	res.startedAt = time.Now()

	// workers
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			for j := range jobs {
				t0 := time.Now()
				err := doOp(ctx, repo, j.op, r, histLimit)
				dt := time.Since(t0)

				atomic.AddUint64(&res.totalOps, 1)
				if j.op == opRead {
					atomic.AddUint64(&res.readOps, 1)
				} else {
					atomic.AddUint64(&res.writeOps, 1)
				}
				if err != nil {
					atomic.AddUint64(&res.errOps, 1)
					continue
				}
				if collect {
					mu.Lock()
					res.latencies = append(res.latencies, dt)
					mu.Unlock()
				}
			}
		}()
	}

	// producer
	go func() {
		defer close(jobs)

		// pattern: rw reads per 1 write
		// e.g. rw=15 => 15 reads then 1 write
		pattern := make([]opType, 0, rw+1)
		for i := 0; i < rw; i++ {
			pattern = append(pattern, opRead)
		}
		pattern = append(pattern, opWrite)
		idx := 0

		rampStart := time.Now()

		for {
			if err := lim.Wait(ctx); err != nil {
				return
			}

			// ramp logic
			if ramp > 0 {
				el := time.Since(rampStart)
				if el < ramp {
					// linear from avgRPS -> peakRPS
					cur := float64(avgRPS) + (float64(peakRPS-avgRPS) * (float64(el) / float64(ramp)))
					lim.SetLimit(rate.Limit(cur))
				} else {
					lim.SetLimit(rate.Limit(peakRPS))
				}
			}

			jobs <- job{op: pattern[idx]}
			idx++
			if idx == len(pattern) {
				idx = 0
			}
		}
	}()

	wg.Wait()
	res.finishedAt = time.Now()
	return res
}

func doOp(ctx context.Context, repo storage.Repository, op opType, r *rand.Rand, histLimit int) error {
	chatID := int64(1 + r.Intn(20000)) // имитация 20k пользователей
	switch op {
	case opRead:
		_, err := repo.ListHistory(ctx, chatID, histLimit)
		return err
	case opWrite:
		tx := fakeTx(r)
		if err := repo.UpsertTx(ctx, tx); err != nil {
			return err
		}
		return repo.AddChatEvent(ctx, chatID, tx.Hash, storage.EventNotify)
	default:
		return nil
	}
}

func fakeTx(r *rand.Rand) storage.TxRecord {
	// уникальный hash
	hash := fmt.Sprintf("0x%064x", r.Uint64()) // упрощённо, но достаточно для теста
	chainID := "1"
	from := fmt.Sprintf("0x%040x", r.Uint64())
	to := fmt.Sprintf("0x%040x", r.Uint64())
	valueWei := "1000000000000000000" // 1 ETH
	gp := "1"
	now := time.Now().UTC()
	bn := uint64(r.Intn(30_000_000))

	return storage.TxRecord{
		Hash:        hash,
		ChainID:     chainID,
		BlockNum:    &bn,
		BlockTime:   &now,
		FromAddr:    from,
		ToAddr:      &to,
		ValueWei:    valueWei,
		Nonce:       uint64(r.Intn(1000)),
		TxType:      0,
		Gas:         21000,
		GasPriceWei: &gp,
	}
}

func printReport(res results) {
	d := res.finishedAt.Sub(res.startedAt)
	total := atomic.LoadUint64(&res.totalOps)
	errs := atomic.LoadUint64(&res.errOps)
	reads := atomic.LoadUint64(&res.readOps)
	writes := atomic.LoadUint64(&res.writeOps)

	fmt.Printf("\n== REPORT ==\n")
	fmt.Printf("duration: %s\n", d)
	fmt.Printf("ops: total=%d read=%d write=%d errors=%d\n", total, reads, writes, errs)
	if d > 0 {
		fmt.Printf("throughput: %.2f ops/s\n", float64(total)/d.Seconds())
	}
	if len(res.latencies) == 0 {
		fmt.Println("no latency samples")
		return
	}
	sort.Slice(res.latencies, func(i, j int) bool { return res.latencies[i] < res.latencies[j] })
	p := func(q float64) time.Duration {
		i := int(q * float64(len(res.latencies)-1))
		return res.latencies[i]
	}
	fmt.Printf("latency p50=%s p95=%s p99=%s max=%s\n",
		p(0.50), p(0.95), p(0.99), res.latencies[len(res.latencies)-1],
	)
}
