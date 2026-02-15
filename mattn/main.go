package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
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

	fmt.Println("=== mattn/go-sqlite3 ===")

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

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_sync=NORMAL&_cache_size=-64000&_busy_timeout=5000&_query_only=1")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(numGoroutines)
	db.SetMaxIdleConns(numGoroutines)

	ctx := context.Background()
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
	done := make(chan struct{}, numGoroutines)
	errChan := make(chan error, numGoroutines)
	idsPerWorker := len(ids) / numGoroutines

	for i := 0; i < numGoroutines; i++ {
		go func(workerID int) {
			defer func() { done <- struct{}{} }()
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

	for i := 0; i < numGoroutines; i++ {
		<-done
	}
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
		DriverName:     "mattn/go-sqlite3",
		Mode:           mode,
		NumGoroutines:  numGoroutines,
		NumReads:       len(ids),
		Duration:       duration.Nanoseconds(),
		ReadsPerSecond: float64(len(ids)) / duration.Seconds(),
	}, nil
}
