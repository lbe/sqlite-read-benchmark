package common

import "context"

// FolderPath represents a single folder path entry
type FolderPath struct {
	ID   int64
	Path string
}

// DBReader is the interface that each driver must implement for benchmarking
type DBReader interface {
	// GetRaw performs a raw SELECT query for a given ID
	GetRaw(ctx context.Context, id int64) (*FolderPath, error)
	// GetPrepared performs a prepared SELECT query for a given ID
	GetPrepared(ctx context.Context, id int64) (*FolderPath, error)
	// Close closes the database connection
	Close() error
}

// BenchmarkConfig holds configuration for the benchmark
type BenchmarkConfig struct {
	DBPath          string
	NumGoroutines   int
	NumReads        int
	TotalRows       int64
}

// BenchmarkResult holds the results of a benchmark run
type BenchmarkResult struct {
	DriverName      string
	Mode            string // "raw" or "prepared"
	NumGoroutines   int
	NumReads        int
	Duration        int64 // nanoseconds
	ReadsPerSecond  float64
}
