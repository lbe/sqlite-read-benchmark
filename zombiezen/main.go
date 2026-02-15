package main

import (
	"fmt"
	"os"
	"time"

	"zombiezen.com/go/sqlite"
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

	fmt.Println("=== zombiezen.com/go/sqlite ===")

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

	// Open connections for each goroutine
	conns := make([]*sqlite.Conn, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		conn, err := sqlite.OpenConn(dbPath, sqlite.OpenReadOnly|sqlite.OpenWAL|sqlite.OpenNoMutex)
		if err != nil {
			for j := 0; j < i; j++ {
				conns[j].Close()
			}
			return nil, fmt.Errorf("failed to open connection: %w", err)
		}
		conns[i] = conn
	}

	// Prepare statements if using prepared mode
	stmts := make([]*sqlite.Stmt, numGoroutines)
	if usePrepared {
		for i := 0; i < numGoroutines; i++ {
			stmt, err := conns[i].Prepare("SELECT id, path FROM folder_paths WHERE id = ?")
			if err != nil {
				for j := 0; j < i; j++ {
					stmts[j].Finalize()
				}
				for j := 0; j < numGoroutines; j++ {
					conns[j].Close()
				}
				return nil, fmt.Errorf("failed to prepare statement: %w", err)
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
			startIdx := workerID * idsPerWorker
			endIdx := startIdx + idsPerWorker
			if workerID == numGoroutines-1 {
				endIdx = len(ids)
			}

			for _, id := range ids[startIdx:endIdx] {
				var err error
				if usePrepared {
					stmt := stmts[workerID]
					stmt.BindInt64(1, id)
					hasRow, stepErr := stmt.Step()
					if stepErr != nil {
						errChan <- stepErr
						return
					}
					if hasRow {
						_ = stmt.GetInt64("id")
						_ = stmt.GetText("path")
					}
					stmt.Reset()
				} else {
					// Raw mode: prepare, bind, step, finalize for each query
					stmt, prepErr := conn.Prepare("SELECT id, path FROM folder_paths WHERE id = ?")
					if prepErr != nil {
						errChan <- prepErr
						return
					}
					stmt.BindInt64(1, id)
					hasRow, stepErr := stmt.Step()
					if stepErr != nil {
						stmt.Finalize()
						errChan <- stepErr
						return
					}
					if hasRow {
						_ = stmt.GetInt64("id")
						_ = stmt.GetText("path")
					}
					stmt.Finalize()
				}
				if err != nil {
					errChan <- err
					return
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
				if conns[i] != nil {
					conns[i].Close()
				}
			}
			return nil, err
		}
	}

	// Cleanup
	for i := 0; i < numGoroutines; i++ {
		if stmts[i] != nil {
			stmts[i].Finalize()
		}
		if conns[i] != nil {
			conns[i].Close()
		}
	}

	return &BenchmarkResult{
		DriverName:     "zombiezen.com/go/sqlite",
		Mode:           mode,
		NumGoroutines:  numGoroutines,
		NumReads:       len(ids),
		Duration:       duration.Nanoseconds(),
		ReadsPerSecond: float64(len(ids)) / duration.Seconds(),
	}, nil
}
