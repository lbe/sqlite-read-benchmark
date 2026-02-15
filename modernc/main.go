package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: %s <db_path> <num_reads> <num_goroutines>\n", os.Args[0])
		os.Exit(1)
	}

	dbPath := os.Args[1]
	numReads := 0
	numGoroutines := 0
	fmt.Sscanf(os.Args[2], "%d", &numReads)
	fmt.Sscanf(os.Args[3], "%d", &numGoroutines)

	var numEntries int64 = 6000000
	ids := make([]int64, numReads)
	for i := 0; i < numReads; i++ {
		ids[i] = RandInt(numEntries) + 1
	}
	ShuffleIDs(ids)

	fmt.Println("=== modernc.org/sqlite ===")

	fmt.Println("  Running raw benchmark...")
	rawResult, err := runBenchmark(dbPath, ids, numGoroutines, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Raw benchmark failed: %v\n", err)
		os.Exit(1)
	}
	PrintResult(rawResult)

	fmt.Println("  Running prepared benchmark...")
	preparedResult, err := runBenchmark(dbPath, ids, numGoroutines, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Prepared benchmark failed: %v\n", err)
		os.Exit(1)
	}
	PrintResult(preparedResult)
}

func runBenchmark(dbPath string, ids []int64, numGoroutines int, usePrepared bool) (*BenchmarkResult, error) {
	mode := "raw"
	if usePrepared {
		mode = "prepared"
	}

	ctx := context.Background()

	// modernc.org/sqlite works best with a single pool and proper connection handling
	// Use very long busy timeout and single connection per goroutine
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_sync=NORMAL&_cache_size=-64000&_busy_timeout=300000&_query_only=1")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Single connection per goroutine, no sharing
	db.SetMaxOpenConns(numGoroutines)
	db.SetMaxIdleConns(numGoroutines)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Get dedicated connections for each worker
	conns := make([]*sql.Conn, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		conn, err := db.Conn(ctx)
		if err != nil {
			for j := 0; j < i; j++ {
				conns[j].Close()
			}
			return nil, err
		}
		conns[i] = conn
	}

	// Small delay to ensure connections are fully established
	time.Sleep(100 * time.Millisecond)

	// Prepare statements if needed
	stmts := make([]*sql.Stmt, numGoroutines)
	if usePrepared {
		for i := 0; i < numGoroutines; i++ {
			stmt, err := conns[i].PrepareContext(ctx, "SELECT id, path FROM folder_paths WHERE id = ?")
			if err != nil {
				for j := 0; j < i; j++ {
					stmts[j].Close()
				}
				for j := 0; j < numGoroutines; j++ {
					conns[j].Close()
				}
				return nil, err
			}
			stmts[i] = stmt
		}
	}

	start := time.Now()
	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines)
	idsPerWorker := len(ids) / numGoroutines

	// Stagger goroutine start to reduce lock contention
	startDelay := time.Millisecond * 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			// Stagger start times
			time.Sleep(time.Duration(workerID) * startDelay)
			
			conn := conns[workerID]
			var stmt *sql.Stmt
			if usePrepared {
				stmt = stmts[workerID]
			}
			startIdx := workerID * idsPerWorker
			endIdx := startIdx + idsPerWorker
			if workerID == numGoroutines-1 {
				endIdx = len(ids)
			}
			for _, id := range ids[startIdx:endIdx] {
				var idOut int64
				var path string
				var err error
				if usePrepared {
					err = stmt.QueryRowContext(ctx, id).Scan(&idOut, &path)
				} else {
					err = conn.QueryRowContext(ctx, "SELECT id, path FROM folder_paths WHERE id = ?", id).Scan(&idOut, &path)
				}
				if err != nil && err != sql.ErrNoRows {
					errChan <- err
					return
				}
				_ = idOut
				_ = path
			}
		}(i)
	}

	wg.Wait()
	close(errChan)
	duration := time.Since(start)

	for err := range errChan {
		if err != nil {
			for i := 0; i < numGoroutines; i++ {
				if stmts[i] != nil {
					stmts[i].Close()
				}
				conns[i].Close()
			}
			return nil, err
		}
	}

	for i := 0; i < numGoroutines; i++ {
		if stmts[i] != nil {
			stmts[i].Close()
		}
		conns[i].Close()
	}

	return &BenchmarkResult{
		DriverName:     "modernc.org/sqlite",
		Mode:           mode,
		NumGoroutines:  numGoroutines,
		NumReads:       len(ids),
		Duration:       duration.Nanoseconds(),
		ReadsPerSecond: float64(len(ids)) / duration.Seconds(),
	}, nil
}
