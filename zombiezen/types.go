package main

import (
	"fmt"
	"time"
)

type BenchmarkResult struct {
	DriverName     string
	Mode           string
	NumGoroutines  int
	NumReads       int
	Duration       int64
	ReadsPerSecond float64
}

func PrintResult(r *BenchmarkResult) {
	fmt.Printf("  %s (%d goroutines): %d reads in %v = %.0f reads/sec\n",
		r.Mode, r.NumGoroutines, r.NumReads, time.Duration(r.Duration), r.ReadsPerSecond)
}
