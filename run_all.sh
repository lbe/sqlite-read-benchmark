#!/bin/bash

if [ $# -lt 3 ]; then
    echo "Usage: $0 <db_path> <num_reads> <num_goroutines>"
    echo "Example: $0 benchmark.db 100000 8"
    exit 1
fi

DB_PATH=$1
NUM_READS=$2
NUM_GOROUTINES=$3

echo "SQLite Driver Benchmark Suite"
echo "=============================="
echo "Database: $DB_PATH"
echo "Reads: $NUM_READS"
echo "Goroutines: $NUM_GOROUTINES"
echo ""

if [ ! -f "$DB_PATH" ]; then
    echo "Error: Database file not found: $DB_PATH"
    echo "Run: go run main.go -db $DB_PATH -entries 6000000"
    exit 1
fi

run_benchmark() {
    local name=$1
    local binary=$2
    
    echo ""
    if ./bin/$binary "$DB_PATH" "$NUM_READS" "$NUM_GOROUTINES"; then
        echo "✓ $name completed"
    else
        echo "✗ $name failed"
    fi
}

run_benchmark "mattn/go-sqlite3" "benchmark_mattn"
run_benchmark "modernc.org/sqlite" "benchmark_modernc"
run_benchmark "github.com/ncruces/go-sqlite3" "benchmark_ncruces"
run_benchmark "crawshaw.io/sqlite" "benchmark_crawshaw"
run_benchmark "zombiezen.com/go/sqlite" "benchmark_zombiezen"
run_benchmark "github.com/glebarez/sqlite" "benchmark_glebarez"

echo ""
echo "=============================="
echo "Benchmark complete!"
