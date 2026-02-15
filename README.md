# SQLite Go Driver Benchmark

This project benchmarks read performance of various Go SQLite drivers comparing raw vs prepared statements.

## Supported Drivers

- [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) - CGO-based driver
- [modernc.org/sqlite](https://modernc.org/sqlite) - Pure Go (transpiled from C)
- [github.com/ncruces/go-sqlite3](https://github.com/ncruces/go-sqlite3) - Pure Go (WASM-based)
- [crawshaw.io/sqlite](https://crawshaw.io/sqlite) - CGO-based low-level driver
- [zombiezen.com/go/sqlite](https://zombiezen.com/go/sqlite) - Pure Go (modernc fork)
- [github.com/glebarez/sqlite](https://github.com/glebarez/sqlite) - GORM driver (uses modernc)

## Project Structure

Each driver is in its own directory as a **completely independent Go module**. This is necessary because:
1. CGO-based drivers (mattn, crawshaw) embed SQLite C code which causes symbol conflicts
2. Some drivers register themselves with the same `database/sql` driver name

```
sqlite-benchmark/
├── main.go                    # Database initialization and setup
├── generator/                 # Mock data generation (shared, no CGO)
├── common/                    # Shared types (no CGO dependencies)
├── mattn/                     # mattn/go-sqlite3 benchmark
├── modernc/                   # modernc.org/sqlite benchmark
├── ncruces/                   # ncruces/go-sqlite3 benchmark
├── crawshaw/                  # crawshaw.io/sqlite benchmark
├── zombiezen/                 # zombiezen.com/go/sqlite benchmark
└── glebarez/                  # glebarez/sqlite (GORM) benchmark
```

## Quick Start

### 1. Initialize the Database

```bash
# Create database with 6 million entries
go run main.go -db benchmark.db -entries 6000000
```

### 2. Build All Benchmark Binaries

```bash
./build.sh
```

This will create binaries in `./bin/`:
- `benchmark_mattn`
- `benchmark_modernc`
- `benchmark_ncruces`
- `benchmark_crawshaw`
- `benchmark_zombiezen`
- `benchmark_glebarez`

### 3. Run Individual Benchmarks

```bash
# Usage: ./bin/benchmark_<driver> <db_path> <num_reads> <num_goroutines>
./bin/benchmark_mattn benchmark.db 100000 8
./bin/benchmark_modernc benchmark.db 100000 8
./bin/benchmark_ncruces benchmark.db 100000 8
./bin/benchmark_crawshaw benchmark.db 100000 8
./bin/benchmark_zombiezen benchmark.db 100000 8
./bin/benchmark_glebarez benchmark.db 100000 8
```

### 4. Run All Benchmarks

```bash
./run_all.sh benchmark.db 100000 8
```

## Configuration

### Database Setup Options (main.go)

```bash
go run main.go -db <path> -entries <count> [-skip-init] [-skip-warmup] [-warmup-only]
```

- `-db`: Database file path (default: `benchmark.db`)
- `-entries`: Number of entries to create (default: `6000000`)
- `-skip-init`: Skip database initialization (use existing DB)
- `-skip-warmup`: Skip file cache warmup
- `-warmup-only`: Only warm up cache, don't create data

### Benchmark Options

Each benchmark binary accepts:
1. Database path
2. Number of reads (e.g., 100000)
3. Number of goroutines (e.g., 8)

## Benchmark Methodology

1. **Database Creation**: Creates `folder_paths` table with unique path strings
2. **Mock Data Generation**: Generates 6 million entries using crypto/rand
3. **Cache Warmup**: Reads all rows sequentially to warm up OS file cache
4. **Benchmark Execution**:
   - Raw benchmark: Each query prepares, executes, and finalizes
   - Prepared benchmark: Statement prepared once, reused with reset/bind
5. **Concurrency**: Each goroutine maintains its own database connection

## Schema

```sql
CREATE TABLE folder_paths (
  id INTEGER PRIMARY KEY,
  path TEXT UNIQUE NOT NULL
);
```

## Example Output

```
=== mattn/go-sqlite3 ===
  Running raw benchmark...
  raw (8 goroutines): 100000 reads in 1.234s = 81037 reads/sec
  Running prepared benchmark...
  prepared (8 goroutines): 100000 reads in 0.987s = 101317 reads/sec

=== modernc.org/sqlite ===
  Running raw benchmark...
  raw (8 goroutines): 100000 reads in 1.456s = 68681 reads/sec
  Running prepared benchmark...
  prepared (8 goroutines): 100000 reads in 1.123s = 89047 reads/sec
```

## Notes

- The CGO-based drivers (mattn, crawshaw) generally offer the best performance
- Pure Go drivers provide better portability (no C compiler required)
- Prepared statements typically show 10-30% performance improvement
- File cache warmup is crucial for accurate benchmark results
