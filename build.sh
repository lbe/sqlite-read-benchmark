#!/bin/bash

set -e

mkdir -p bin

echo "Building benchmark binaries..."

cd mattn
go mod tidy
go build -o ../bin/benchmark_mattn .
echo "✓ mattn/go-sqlite3"
cd ..

cd modernc
go mod tidy
go build -o ../bin/benchmark_modernc .
echo "✓ modernc.org/sqlite"
cd ..

cd ncruces
go mod tidy
go build -o ../bin/benchmark_ncruces .
echo "✓ github.com/ncruces/go-sqlite3"
cd ..

cd crawshaw
go mod tidy
go build -o ../bin/benchmark_crawshaw .
echo "✓ crawshaw.io/sqlite"
cd ..

cd zombiezen
go mod tidy
go build -o ../bin/benchmark_zombiezen .
echo "✓ zombiezen.com/go/sqlite"
cd ..

cd glebarez
go mod tidy
go build -o ../bin/benchmark_glebarez .
echo "✓ github.com/glebarez/sqlite"
cd ..

echo ""
echo "All binaries built successfully in ./bin/"
