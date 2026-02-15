package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
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

	fmt.Println("=== crawshaw.io/sqlite ===")

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

	// Open connection pool
	pool, err := sqlitex.Open(dbPath, 0, numGoroutines)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer pool.Close()

	ctx := context.Background()

	// Get connections from pool for each worker
	conns := make([]*sqlite.Conn, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		conn := pool.Get(ctx)
		if conn == nil {
			for j := 0; j < i; j++ {
				pool.Put(conns[j])
			}
			return nil, fmt.Errorf("failed to get connection from pool")
		}
		conns[i] = conn
	}

	// Prepare statements for each worker if using prepared mode
	stmts := make([]*sqlite.Stmt, numGoroutines)
	if usePrepared {
		for i := 0; i < numGoroutines; i++ {
			stmt := conns[i].Prep("SELECT id, path FROM folder_paths WHERE id = ?")
			if stmt == nil {
				for j := 0; j < i; j++ {
					stmts[j].Finalize()
				}
				for j := 0; j < numGoroutines; j++ {
					pool.Put(conns[j])
				}
				return nil, fmt.Errorf("failed to prepare statement")
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
			var stmt *sqlite.Stmt
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
				var hasRow bool

				if usePrepared {
					// Reset statement for reuse
					stmt.Reset()
					stmt.BindInt64(1, id)
					var stepErr error
					hasRow, stepErr = stmt.Step()
					if stepErr != nil {
						errChan <- stepErr
						return
					}
					if hasRow {
						idOut = stmt.GetInt64("id")
						path = stmt.GetText("path")
					}
				} else {
					// Raw query - prepare, bind, step, finalize
					stmt := conn.Prep("SELECT id, path FROM folder_paths WHERE id = ?")
					if stmt == nil {
						errChan <- fmt.Errorf("failed to prepare statement")
						return
					}
					stmt.BindInt64(1, id)
					var stepErr error
					hasRow, stepErr = stmt.Step()
					if stepErr != nil {
						stmt.Finalize()
						errChan <- stepErr
						return
					}
					if hasRow {
						idOut = stmt.GetInt64("id")
						path = stmt.GetText("path")
					}
					stmt.Finalize()
				}

				if hasRow {
					_ = idOut
					_ = path
				}
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
			// Cleanup
			for i := 0; i < numGoroutines; i++ {
				if stmts[i] != nil {
					stmts[i].Finalize()
				}
				pool.Put(conns[i])
			}
			return nil, err
		}
	}

	// Cleanup
	for i := 0; i < numGoroutines; i++ {
		if stmts[i] != nil {
			stmts[i].Finalize()
		}
		pool.Put(conns[i])
	}

	return &BenchmarkResult{
		DriverName:     "crawshaw.io/sqlite",
		Mode:           mode,
		NumGoroutines:  numGoroutines,
		NumReads:       len(ids),
		Duration:       duration.Nanoseconds(),
		ReadsPerSecond: float64(len(ids)) / duration.Seconds(),
	}, nil
}
