package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// FolderPath represents a folder path in the database
type FolderPath struct {
	ID   int64  `gorm:"column:id;primaryKey"`
	Path string `gorm:"column:path"`
}

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

	fmt.Println("=== github.com/glebarez/sqlite ===")

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

	db, err := gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL&_sync=NORMAL&_cache_size=-64000&_busy_timeout=5000"), &gorm.Config{
		SkipDefaultTransaction: true,
		PrepareStmt:            usePrepared,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}
	defer sqlDB.Close()

	sqlDB.SetMaxOpenConns(numGoroutines)
	sqlDB.SetMaxIdleConns(numGoroutines)

	start := time.Now()
	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines)
	idsPerWorker := len(ids) / numGoroutines

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			startIdx := workerID * idsPerWorker
			endIdx := startIdx + idsPerWorker
			if workerID == numGoroutines-1 {
				endIdx = len(ids)
			}

			for _, id := range ids[startIdx:endIdx] {
				var result FolderPath
				var err error
				if usePrepared {
					err = db.First(&result, id).Error
				} else {
					err = db.Raw("SELECT id, path FROM folder_paths WHERE id = ?", id).Scan(&result).Error
				}
				if err != nil {
					errChan <- err
					return
				}
				_ = result.ID
				_ = result.Path
			}
		}(i)
	}

	wg.Wait()
	close(errChan)
	duration := time.Since(start)

	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	return &BenchmarkResult{
		DriverName:     "github.com/glebarez/sqlite",
		Mode:           mode,
		NumGoroutines:  numGoroutines,
		NumReads:       len(ids),
		Duration:       duration.Nanoseconds(),
		ReadsPerSecond: float64(len(ids)) / duration.Seconds(),
	}, nil
}
