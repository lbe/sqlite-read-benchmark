package common

import (
	"fmt"
	"sync"
	"time"
)

// BenchmarkRunner is a function that runs a benchmark for a specific driver
type BenchmarkRunner func(dbPath string, ids []int64, numGoroutines int) (*BenchmarkResult, *BenchmarkResult, error)

// RunBenchmarkForDriver runs both raw and prepared benchmarks for a driver
func RunBenchmarkForDriver(name string, runner BenchmarkRunner, dbPath string, ids []int64, numGoroutines int) error {
	fmt.Printf("\n=== %s ===\n", name)
	
	rawResult, preparedResult, err := runner(dbPath, ids, numGoroutines)
	if err != nil {
		return fmt.Errorf("benchmark failed for %s: %w", name, err)
	}
	
	PrintResult(rawResult)
	PrintResult(preparedResult)
	
	improvement := ((preparedResult.ReadsPerSecond - rawResult.ReadsPerSecond) / rawResult.ReadsPerSecond) * 100
	fmt.Printf("Prepared statement improvement: %.1f%%\n", improvement)
	
	return nil
}

// PrintResult prints a benchmark result
func PrintResult(r *BenchmarkResult) {
	fmt.Printf("  %s (%d goroutines): %d reads in %v = %.0f reads/sec\n",
		r.Mode,
		r.NumGoroutines,
		r.NumReads,
		time.Duration(r.Duration),
		r.ReadsPerSecond,
	)
}

// WorkerFunc is the function signature for worker goroutines
type WorkerFunc func(ids []int64, workerID int) error

// RunParallelBenchmark runs a benchmark with N goroutines
func RunParallelBenchmark(ids []int64, numGoroutines int, worker WorkerFunc) (time.Duration, error) {
	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines)
	
	idsPerWorker := len(ids) / numGoroutines
	
	start := time.Now()
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			startIdx := workerID * idsPerWorker
			endIdx := startIdx + idsPerWorker
			if workerID == numGoroutines-1 {
				endIdx = len(ids)
			}
			
			if err := worker(ids[startIdx:endIdx], workerID); err != nil {
				errChan <- err
			}
		}(i)
	}
	
	wg.Wait()
	close(errChan)
	
	duration := time.Since(start)
	
	for err := range errChan {
		if err != nil {
			return duration, err
		}
	}
	
	return duration, nil
}

// ShuffleIDs shuffles the IDs randomly
func ShuffleIDs(ids []int64) {
	for i := len(ids) - 1; i > 0; i-- {
		j := int(RandInt(int64(i + 1)))
		ids[i], ids[j] = ids[j], ids[i]
	}
}
