package generator

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PathGenerator generates unique folder paths
type PathGenerator struct {
	mu      sync.Mutex
	counter int64
	seen    map[string]bool
}

// NewPathGenerator creates a new path generator
func NewPathGenerator() *PathGenerator {
	return &PathGenerator{
		seen: make(map[string]bool),
	}
}

// Generate creates a unique folder path
func (g *PathGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	var path string
	for {
		path = generateRandomPath()
		if !g.seen[path] {
			g.seen[path] = true
			g.counter++
			break
		}
	}
	return path
}

// generateRandomPath creates a random file system-like path
func generateRandomPath() string {
	depth := 3 + int(randInt(4)) // depth 3-6
	parts := make([]string, 0, depth+1)
	
	// Root
	parts = append(parts, "/home")
	
	// User directories
	parts = append(parts, randomString(4+int(randInt(8))))
	
	// Subdirectories
	for i := 0; i < depth; i++ {
		parts = append(parts, randomString(3+int(randInt(10))))
	}
	
	// Sometimes add a file at the end
	if randInt(2) == 1 {
		parts = append(parts, randomString(5+int(randInt(10)))+".txt")
	}
	
	return filepath.Join(parts...)
}

// randomString generates a random string of given length
func randomString(length int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789_-"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		b[i] = letters[n.Int64()]
	}
	return string(b)
}

// randInt returns a random int in [0, n)
func randInt(n int64) int64 {
	val, _ := rand.Int(rand.Reader, big.NewInt(n))
	return val.Int64()
}

// RandInt is exported version of randInt
func RandInt(n int64) int64 {
	return randInt(n)
}

// GenerateMockData generates N unique folder paths with progress display
func GenerateMockData(count int64) []string {
	paths := make([]string, count)
	gen := NewPathGenerator()
	
	fmt.Printf("Generating %d mock entries...\n\n", count)
	fmt.Println("Time        Generated    Remaining    Complete    Rate")
	fmt.Println("=========================================================")
	
	startTime := time.Now()
	progressInterval := int64(100000)
	if count < progressInterval {
		progressInterval = count / 10
		if progressInterval < 1 {
			progressInterval = 1
		}
	}
	
	for i := int64(0); i < count; i++ {
		path := gen.Generate()
		paths[i] = fmt.Sprintf("/data/path_%d/%s", i, path)
		
		// Progress reporting for large batches
		if (i+1)%progressInterval == 0 || i == count-1 {
			currentTime := time.Now()
			generated := i + 1
			remaining := count - generated
			elapsed := currentTime.Sub(startTime)
			rate := float64(generated) / elapsed.Seconds()
			
			timeStr := currentTime.Format("15:04:05")
			
			fmt.Printf("%s  %10d  %11d  %7.2f%%  %10.0f\n",
				timeStr,
				generated,
				remaining,
				float64(generated)*100/float64(count),
				rate,
			)
			

		}
	}
	
	totalDuration := time.Since(startTime)
	fmt.Println("=========================================================")
	fmt.Printf("Generation complete: %v, Average rate: %.0f paths/sec\n\n", 
		totalDuration.Round(time.Second), float64(count)/totalDuration.Seconds())
	
	return paths
}

// SimplePathGenerator is a simpler generator without the mock framework overhead
// for when we just need fast path generation
type SimplePathGenerator struct {
	prefix string
	mu     sync.Mutex
	count  int64
}

// NewSimplePathGenerator creates a simple path generator
func NewSimplePathGenerator(prefix string) *SimplePathGenerator {
	return &SimplePathGenerator{
		prefix: prefix,
	}
}

// Generate generates a unique path
func (g *SimplePathGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.count++
	return fmt.Sprintf("%s/path_%d_%s", g.prefix, g.count, randomString(16))
}

// UniquePathGenerator ensures all paths are unique using a map
type UniquePathGenerator struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

// NewUniquePathGenerator creates a new unique path generator
func NewUniquePathGenerator() *UniquePathGenerator {
	return &UniquePathGenerator{
		seen: make(map[string]struct{}),
	}
}

// Generate creates a unique path
func (g *UniquePathGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	var path string
	for {
		sb := strings.Builder{}
		sb.WriteString("/home/")
		sb.WriteString(randomString(8))
		sb.WriteString("/documents/")
		sb.WriteString(randomString(12))
		sb.WriteString("/")
		sb.WriteString(randomString(16))
		path = sb.String()
		
		if _, exists := g.seen[path]; !exists {
			g.seen[path] = struct{}{}
			break
		}
	}
	return path
}
