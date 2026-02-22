# SQLite Go Driver Benchmark

This project benchmarks read performance of various Go SQLite drivers comparing raw vs prepared statements.

I created this project after discovering in cpu profiles that [modernc.org/sqlite](https://modernc.org/sqlite) was apparently re-preparing already prepared statements. The of this benchmark support this observation.
I have subsequently found an issue - [Optimize prepared statements?](https://gitlab.com/cznic/sqlite/-/issues?sort=created_date&state=opened&search=prepared&first_page_size=20&show=eyJpaWQiOiIyMzYiLCJmdWxsX3BhdGgiOiJjem5pYy9zcWxpdGUiLCJpZCI6MTc3OTgwMjkzfQ%3D%3D)
reporting this same behavior.

---

**UPDATE:** Since publishing the original version of this benchmark, I have discovered that this benchmark did not
use the latest version of all drivers.  Namely in the case of [modernc.org/sqlite](https://modernc.org/sqlite) it used version
1.35.0.  I also discovered that several days after my original publishing of this bechnmark that issue [Optimize prepared statements?](https://gitlab.com/cznic/sqlite/-/issues?sort=created_date&state=opened&search=prepared&first_page_size=20&show=eyJpaWQiOiIyMzYiLCJmdWxsX3BhdGgiOiJjem5pYy9zcWxpdGUiLCJpZCI6MTc3OTgwMjkzfQ%3D%3D) has been closed with a message that
this defect was addressed in a commit made on December 7, 2025.  I have updated all dependencies in all of the benchmark code
to the very latest versions available as of February 22, 2026. The current version 1.46.1 of [modernc.org/sqlite](https://modernc.org/sqlite) now shows the expected behavior that prepared are faster than raw, 39% faster in this benchmark.  Thank you to 
the modernc developer(s) for addressing this.

In this specific benchmark, the [github.com/ncruces/go-sqlite3](https://github.com/ncruces/go-sqlite3) driver still
performs 314% faster for my needs than [modernc.org/sqlite](https://modernc.org/sqlite).

Additionally, I updated the go version to 1.26.0. As such the benchmarks now reflect the performance improvements 
made in this verion of CGo.

---


I have replaced the [modernc.org/sqlite](https://modernc.org/sqlite) with [github.com/ncruces/go-sqlite3](https://github.com/ncruces/go-sqlite3) in my application in order to maintain my CGO free goal. The benchmark indicated that would improve performance by a factor of 5+. In the actual
application, it resulted in reducing the runtime of a long process from 33 minutes to 1 minute.

Please note that as with all benchmrks, the results are only as indicative as the benchmark matches your actual use case.
The only way to be sure that the results are applicable to your case is to benchmark your case!

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

**Update:** February 22, 2026
```
SQLite Driver Benchmark Suite
==============================
Database: benchmark.db
Reads: 100000
Goroutines: 22


=== mattn/go-sqlite3 ===
  Running raw benchmark...
  raw (22 goroutines): 100000 reads in 4.317282444s = 23163 reads/sec
  Running prepared benchmark...
  prepared (22 goroutines): 100000 reads in 2.277671663s = 43904 reads/sec
✓ mattn/go-sqlite3 completed

=== modernc.org/sqlite ===
  Running raw benchmark...
  raw (22 goroutines): 100000 reads in 2.585778403s = 38673 reads/sec
  Running prepared benchmark...
  prepared (22 goroutines): 100000 reads in 1.865742162s = 53598 reads/sec
✓ modernc.org/sqlite completed

=== github.com/ncruces/go-sqlite3 ===
  Running raw benchmark...
  raw (22 goroutines): 100000 reads in 679.047214ms = 147265 reads/sec
  Running prepared benchmark...
  prepared (22 goroutines): 100000 reads in 592.568508ms = 168757 reads/sec
✓ github.com/ncruces/go-sqlite3 completed

=== crawshaw.io/sqlite ===
  Running raw benchmark...
  raw (22 goroutines): 100000 reads in 3.393184652s = 29471 reads/sec
  Running prepared benchmark...
  prepared (22 goroutines): 100000 reads in 468.828425ms = 213298 reads/sec
✓ crawshaw.io/sqlite completed

=== zombiezen.com/go/sqlite ===
  Running raw benchmark...
  raw (22 goroutines): 100000 reads in 15.38200088s = 6501 reads/sec
  Running prepared benchmark...
  prepared (22 goroutines): 100000 reads in 1.682900317s = 59421 reads/sec
✓ zombiezen.com/go/sqlite completed

=== github.com/glebarez/sqlite ===
  Running raw benchmark...
  raw (22 goroutines): 100000 reads in 4.927667012s = 20294 reads/sec
  Running prepared benchmark...
  prepared (22 goroutines): 100000 reads in 5.210097329s = 19193 reads/sec
✓ github.com/glebarez/sqlite completed

==============================
Benchmark complete!
```

**Original Results:**
```
SQLite Driver Benchmark Suite
==============================
Database: benchmark.db
Reads: 100000
Goroutines: 22


=== mattn/go-sqlite3 ===
  Running raw benchmark...
  raw (22 goroutines): 100000 reads in 4.198395945s = 23819 reads/sec
  Running prepared benchmark...
  prepared (22 goroutines): 100000 reads in 2.319155183s = 43119 reads/sec
✓ mattn/go-sqlite3 completed

=== modernc.org/sqlite ===
  Running raw benchmark...
  raw (22 goroutines): 100000 reads in 4.37563765s = 22854 reads/sec
  Running prepared benchmark...
  prepared (22 goroutines): 100000 reads in 4.154677353s = 24069 reads/sec
✓ modernc.org/sqlite completed

=== github.com/ncruces/go-sqlite3 ===
  Running raw benchmark...
  raw (22 goroutines): 100000 reads in 713.712971ms = 140112 reads/sec
  Running prepared benchmark...
  prepared (22 goroutines): 100000 reads in 626.007348ms = 159743 reads/sec
✓ github.com/ncruces/go-sqlite3 completed

=== crawshaw.io/sqlite ===
  Running raw benchmark...
  raw (22 goroutines): 100000 reads in 3.376629975s = 29615 reads/sec
  Running prepared benchmark...
  prepared (22 goroutines): 100000 reads in 403.347256ms = 247925 reads/sec
✓ crawshaw.io/sqlite completed

=== zombiezen.com/go/sqlite ===
  Running raw benchmark...
  raw (22 goroutines): 100000 reads in 14.656310718s = 6823 reads/sec
  Running prepared benchmark...
  prepared (22 goroutines): 100000 reads in 2.482263816s = 40286 reads/sec
✓ zombiezen.com/go/sqlite completed

=== github.com/glebarez/sqlite ===
  Running raw benchmark...
  raw (22 goroutines): 100000 reads in 4.924050485s = 20308 reads/sec
  Running prepared benchmark...
  prepared (22 goroutines): 100000 reads in 6.02162578s = 16607 reads/sec
✓ github.com/glebarez/sqlite completed

==============================
Benchmark complete!
```
