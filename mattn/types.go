package main

import (
	"fmt"
	"time"
)

// BenchmarkResult holds the results of a benchmark run
type BenchmarkResult struct {
	DriverName     string
	Mode           string // "raw" or "prepared"
	NumGoroutines  int
	NumReads       int
	Duration       int64 // nanoseconds
	ReadsPerSecond float64
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
