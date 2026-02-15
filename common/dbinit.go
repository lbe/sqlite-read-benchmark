package common

import (
	"database/sql"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"sqlite-benchmark/generator"

	_ "github.com/mattn/go-sqlite3"
)

const (
	SchemaSQL = `
CREATE TABLE IF NOT EXISTS folder_paths (
  id INTEGER PRIMARY KEY,
  path TEXT UNIQUE NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_folder_paths_path ON folder_paths(path);
`
	ChannelBufferSize = 50000
	BatchSize         = 25000
	ProgressInterval  = 100000
)

// InitDatabase creates and populates the database with mock data
func InitDatabase(dbPath string, numEntries int64) error {
	// Remove existing database
	os.Remove(dbPath)
	os.Remove(dbPath + "-shm")
	os.Remove(dbPath + "-wal")

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_sync=NORMAL&_cache_size=-64000&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Single connection for writing
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Create schema
	if _, err := db.Exec(SchemaSQL); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	fmt.Printf("Inserting %d entries (generation and insertion run in parallel)...\n\n", numEntries)
	fmt.Println("Time        Inserted    Remaining    Complete    Rate        ETA")
	fmt.Println("================================================================")

	// Channel to stream generated paths
	pathChan := make(chan string, ChannelBufferSize)
	
	// Counter for progress tracking
	var insertedCount int64 = 0
	startTime := time.Now()
	
	// Start single writer goroutine
	var writerWg sync.WaitGroup
	writerWg.Add(1)
	var writerErr error
	go func() {
		defer writerWg.Done()
		writerErr = writePaths(db, numEntries, pathChan, &insertedCount, startTime)
	}()

	// Start multiple generator goroutines
	numGenerators := runtime.NumCPU()
	var genWg sync.WaitGroup
	genWg.Add(numGenerators)
	
	entriesPerGenerator := int(numEntries) / numGenerators
	for g := 0; g < numGenerators; g++ {
		go func(generatorID int) {
			defer genWg.Done()
			
			startID := int64(generatorID * entriesPerGenerator)
			endID := startID + int64(entriesPerGenerator)
			if generatorID == numGenerators-1 {
				endID = numEntries
			}
			
			gen := generator.NewPathGenerator()
			for i := startID; i < endID; i++ {
				path := gen.Generate()
				uniquePath := fmt.Sprintf("/data/path_%d/%s", i, path)
				pathChan <- uniquePath
			}
		}(g)
	}

	// Wait for all generators to finish, then close channel
	genWg.Wait()
	close(pathChan)
	
	// Wait for writer to finish
	writerWg.Wait()
	
	if writerErr != nil {
		return fmt.Errorf("writer failed: %w", writerErr)
	}

	totalDuration := time.Since(startTime)
	fmt.Println("================================================================")
	fmt.Printf("Total time: %v, Average rate: %.0f inserts/sec\n", 
		totalDuration.Round(time.Second), float64(numEntries)/totalDuration.Seconds())

	// Create index for better read performance
	fmt.Println("\nCreating indexes...")
	if _, err := db.Exec("PRAGMA optimize"); err != nil {
		return fmt.Errorf("failed to optimize: %w", err)
	}

	// Verify count
	var count int64
	if err := db.QueryRow("SELECT COUNT(*) FROM folder_paths").Scan(&count); err != nil {
		return fmt.Errorf("failed to count rows: %w", err)
	}
	fmt.Printf("Database initialized with %d entries\n", count)

	return nil
}

func writePaths(db *sql.DB, total int64, pathChan <-chan string, counter *int64, startTime time.Time) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	
	stmt, err := tx.Prepare("INSERT INTO folder_paths (id, path) VALUES (?, ?)")
	if err != nil {
		tx.Rollback()
		return err
	}

	batchCount := 0
	id := int64(0)
	
	for path := range pathChan {
		id++
		batchCount++
		if _, err := stmt.Exec(id, path); err != nil {
			stmt.Close()
			tx.Rollback()
			return err
		}
		
		// Update counter atomically
		current := atomic.AddInt64(counter, 1)
		
		// Commit batch periodically
		if batchCount >= BatchSize {
			if err := stmt.Close(); err != nil {
				tx.Rollback()
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}
			
			// Print progress
			if current%ProgressInterval == 0 || current >= total {
				printProgress(current, total, startTime)
			}
			
			// Start new transaction
			tx, err = db.Begin()
			if err != nil {
				return err
			}
			stmt, err = tx.Prepare("INSERT INTO folder_paths (id, path) VALUES (?, ?)")
			if err != nil {
				tx.Rollback()
				return err
			}
			batchCount = 0
		}
	}
	
	// Close statement and commit final batch
	if err := stmt.Close(); err != nil {
		tx.Rollback()
		return err
	}
	if batchCount > 0 {
		if err := tx.Commit(); err != nil {
			return err
		}
		current := atomic.LoadInt64(counter)
		printProgress(current, total, startTime)
	} else {
		tx.Rollback()
	}
	
	return nil
}

func printProgress(inserted int64, total int64, startTime time.Time) {
	currentTime := time.Now()
	elapsed := currentTime.Sub(startTime)
	rate := float64(inserted) / elapsed.Seconds()
	
	remaining := total - inserted
	etaSeconds := float64(remaining) / rate
	eta := time.Duration(etaSeconds) * time.Second
	
	timeStr := currentTime.Format("15:04:05")
	
	fmt.Printf("%s  %10d  %11d  %7.2f%%  %10.0f  %v\n",
		timeStr,
		inserted,
		remaining,
		float64(inserted)*100/float64(total),
		rate,
		eta.Round(time.Second),
	)
}

// WarmupCache reads the entire database to warm up the file cache
func WarmupCache(dbPath string) error {
	fmt.Println("Warming up file cache...")
	
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database for warmup: %w", err)
	}
	defer db.Close()

	start := time.Now()
	
	// Read all rows sequentially to warm up cache
	rows, err := db.Query("SELECT id, path FROM folder_paths")
	if err != nil {
		return fmt.Errorf("failed to query for warmup: %w", err)
	}
	defer rows.Close()

	var count int64
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			return err
		}
		count++
	}
	
	if err := rows.Err(); err != nil {
		return err
	}
	
	fmt.Printf("Cache warmup complete: %d rows read in %v\n", count, time.Since(start))
	return nil
}

// GetRandomIDs generates N random IDs for querying
func GetRandomIDs(count int, maxID int64) []int64 {
	ids := make([]int64, count)
	for i := 0; i < count; i++ {
		ids[i] = int64(i)%maxID + 1
	}
	
	// Shuffle the IDs
	for i := len(ids) - 1; i > 0; i-- {
		j := int(generator.RandInt(int64(i + 1)))
		ids[i], ids[j] = ids[j], ids[i]
	}
	
	return ids
}

// RandInt wraps generator.RandInt
func RandInt(n int64) int64 {
	return generator.RandInt(n)
}
