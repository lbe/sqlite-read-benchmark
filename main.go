package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"sqlite-benchmark/common"
)

const (
	DefaultDBPath        = "benchmark.db"
	DefaultNumEntries    = 6000000
	DefaultNumReads      = 100000
	DefaultNumGoroutines = 8
)

func main() {
	var (
		dbPath        = flag.String("db", DefaultDBPath, "Path to the SQLite database file")
		numEntries    = flag.Int64("entries", DefaultNumEntries, "Number of entries to create in the database")
		numReads      = flag.Int("reads", DefaultNumReads, "Number of random rows to read")
		numGoroutines = flag.Int("goroutines", DefaultNumGoroutines, "Number of goroutines for concurrent reads")
		skipInit      = flag.Bool("skip-init", false, "Skip database initialization")
		skipWarmup    = flag.Bool("skip-warmup", false, "Skip file cache warmup")
		warmupOnly    = flag.Bool("warmup-only", false, "Only warm up cache, don't init")
	)
	flag.Parse()

	if *warmupOnly {
		fmt.Println("Warming up file cache...")
		if err := common.WarmupCache(*dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to warm up cache: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Println("SQLite Driver Benchmark - Database Setup")
	fmt.Println("=========================================")
	fmt.Printf("Database: %s\n", *dbPath)
	fmt.Printf("Entries: %d\n", *numEntries)
	fmt.Printf("CPU cores: %d\n\n", runtime.NumCPU())

	// Initialize database if needed
	if !*skipInit {
		if err := common.InitDatabase(*dbPath, *numEntries); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
			os.Exit(1)
		}
	}

	// Warm up file cache
	if !*skipWarmup {
		if err := common.WarmupCache(*dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to warm up cache: %v\n", err)
			os.Exit(1)
		}
	}

	// Generate random IDs for reading (this is used by the individual benchmark binaries)
	fmt.Printf("\nGenerating %d random IDs for reading...\n", *numReads)
	ids := make([]int64, *numReads)
	for i := 0; i < *numReads; i++ {
		ids[i] = common.RandInt(*numEntries) + 1
	}
	common.ShuffleIDs(ids)

	fmt.Println("\nDatabase setup complete!")
	fmt.Printf("You can now run individual benchmark binaries:\n")
	fmt.Printf("  ./bin/benchmark_mattn %s %d %d\n", *dbPath, *numReads, *numGoroutines)
	fmt.Printf("  ./bin/benchmark_modernc %s %d %d\n", *dbPath, *numReads, *numGoroutines)
	fmt.Printf("  ./bin/benchmark_ncruces %s %d %d\n", *dbPath, *numReads, *numGoroutines)
	fmt.Printf("  ./bin/benchmark_crawshaw %s %d %d\n", *dbPath, *numReads, *numGoroutines)
	fmt.Printf("  ./bin/benchmark_zombiezen %s %d %d\n", *dbPath, *numReads, *numGoroutines)
	fmt.Printf("  ./bin/benchmark_glebarez %s %d %d\n", *dbPath, *numReads, *numGoroutines)
}
