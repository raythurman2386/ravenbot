package db

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

func BenchmarkConcurrentReadWrite(b *testing.B) {
	dbPath := "test_bench.db"
	os.Remove(dbPath)
	os.Remove(dbPath + "-wal")
	os.Remove(dbPath + "-shm")

	db, err := InitDB(dbPath)
	if err != nil {
		b.Fatalf("failed to init db: %v", err)
	}
	defer func() {
		db.Close()
		os.Remove(dbPath)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	}()

	var writeErrors int64
	var readErrors int64
	var firstErr error
	var once sync.Once

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%5 == 0 {
				// Write
				_, err := db.Exec("INSERT INTO headlines (url, title) VALUES (?, ?)",
					fmt.Sprintf("http://example.com/%d-%p", i, pb), "title")
				if err != nil {
					atomic.AddInt64(&writeErrors, 1)
					once.Do(func() {
						firstErr = err
					})
				}
			} else {
				// Read
				rows, err := db.Query("SELECT id, url, title FROM headlines LIMIT 10")
				if err != nil {
					atomic.AddInt64(&readErrors, 1)
					once.Do(func() {
						firstErr = err
					})
				} else {
					for rows.Next() {
						var id int
						var url, title string
						_ = rows.Scan(&id, &url, &title)
					}
					rows.Close()
				}
			}
			i++
		}
	})
	b.StopTimer()

	if writeErrors > 0 || readErrors > 0 {
		b.Logf("Write errors: %d, Read errors: %d, First error: %v", writeErrors, readErrors, firstErr)
	}
}
