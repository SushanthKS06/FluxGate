package tests

import (
	"sync"
	"testing"

	"api-gateway/internal/loadbalancer"
)

func TestRoundRobin_Next(t *testing.T) {
	upstreams := []string{"http://upstream1:8080", "http://upstream2:8080", "http://upstream3:8080"}
	rr := loadbalancer.NewRoundRobin(upstreams)

	// Test basic round-robin
	results := make(map[string]int)
	for i := 0; i < 9; i++ {
		result := rr.Next()
		results[result]++
	}

	// Each upstream should be hit exactly 3 times
	for _, upstream := range upstreams {
		if results[upstream] != 3 {
			t.Errorf("Expected %s to be hit 3 times, got %d", upstream, results[upstream])
		}
	}
}

func TestRoundRobin_Concurrent(t *testing.T) {
	upstreams := []string{"http://upstream1:8080", "http://upstream2:8080"}
	rr := loadbalancer.NewRoundRobin(upstreams)

	var wg sync.WaitGroup
	results := make(chan string, 1000)

	// Simulate concurrent access
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- rr.Next()
		}()
	}

	wg.Wait()
	close(results)

	// Count results
	count1, count2 := 0, 0
	for r := range results {
		if r == upstreams[0] {
			count1++
		} else {
			count2++
		}
	}

	// Should be roughly equal (500 each)
	if count1 < 400 || count1 > 600 {
		t.Errorf("Expected roughly 500 each, got %d and %d", count1, count2)
	}
}

func TestRoundRobin_Empty(t *testing.T) {
	rr := loadbalancer.NewRoundRobin([]string{})
	
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic with empty upstreams")
		}
	}()
	rr.Next()
}
