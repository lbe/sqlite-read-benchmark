package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
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

	fmt.Println("=== github.com/ncruces/go-sqlite3 ===")

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

	// Open separate database connections for each goroutine
	dbs := make([]*sql.DB, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		db, err := sql.Open("sqlite3", "file:"+dbPath+"?_pragma=journal_mode(WAL)&_pragma=sync(NORMAL)&_pragma=cache_size=-64000&_pragma=busy_timeout(5000)")
		if err != nil {
			for j := 0; j < i; j++ {
				dbs[j].Close()
			}
			return nil, fmt.Errorf("failed to open database: %w", err)
		}
		if err := db.Ping(); err != nil {
			for j := 0; j <= i; j++ {
				if dbs[j] != nil {
					dbs[j].Close()
				}
			}
			return nil, fmt.Errorf("failed to ping database: %w", err)
		}
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		dbs[i] = db
	}
	defer func() {
		for _, db := range dbs {
			if db != nil {
				db.Close()
			}
		}
	}()

	// Get connections and prepare statements
	stmts := make([]*sql.Stmt, numGoroutines)
	if usePrepared {
		for i := 0; i < numGoroutines; i++ {
			stmt, err := dbs[i].PrepareContext(ctx, "SELECT id, path FROM folder_paths WHERE id = ?")
			if err != nil {
				for j := 0; j < i; j++ {
					stmts[j].Close()
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

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			db := dbs[workerID]
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
					err = db.QueryRowContext(ctx, "SELECT id, path FROM folder_paths WHERE id = ?", id).Scan(&idOut, &path)
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
			if usePrepared {
				for i := 0; i < numGoroutines; i++ {
					if stmts[i] != nil {
						stmts[i].Close()
					}
				}
			}
			return nil, err
		}
	}

	if usePrepared {
		for i := 0; i < numGoroutines; i++ {
			if stmts[i] != nil {
				stmts[i].Close()
			}
		}
	}

	return &BenchmarkResult{
		DriverName:     "github.com/ncruces/go-sqlite3",
		Mode:           mode,
		NumGoroutines:  numGoroutines,
		NumReads:       len(ids),
		Duration:       duration.Nanoseconds(),
		ReadsPerSecond: float64(len(ids)) / duration.Seconds(),
	}, nil
}
