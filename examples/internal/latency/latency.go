// Package latency measures warmed local work only. It never submits transactions.
package latency

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"
)

type Config struct {
	Warmup  int `json:"warmup"`
	Samples int `json:"samples"`
	Batch   int `json:"batch"`
}
type Result struct {
	Name        string  `json:"name"`
	P50NS       uint64  `json:"p50_ns"`
	P95NS       uint64  `json:"p95_ns"`
	P99NS       uint64  `json:"p99_ns"`
	BytesPerOp  float64 `json:"bytes_per_op"`
	AllocsPerOp float64 `json:"allocs_per_op"`
}

func Load(path string) (Config, error) {
	var c Config
	raw, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(raw, &c); e != nil {
		return c, e
	}
	if c.Warmup < 1 || c.Warmup > 100000 || c.Samples < 100 || c.Samples > 100000 || c.Batch < 1 || c.Batch > 1000 {
		return c, fmt.Errorf("require warmup1..100000 samples100..100000 batch1..1000")
	}
	return c, nil
}
func Measure(name string, c Config, work func() error) (Result, error) {
	result := Result{Name: name}
	for i := 0; i < c.Warmup; i++ {
		if e := work(); e != nil {
			return result, e
		}
	}
	times := make([]uint64, c.Samples)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := range times {
		start := time.Now()
		for j := 0; j < c.Batch; j++ {
			if e := work(); e != nil {
				return result, e
			}
		}
		times[i] = uint64(time.Since(start).Nanoseconds()) / uint64(c.Batch)
	}
	runtime.ReadMemStats(&after)
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	result.P50NS = times[(len(times)-1)*50/100]
	result.P95NS = times[(len(times)-1)*95/100]
	result.P99NS = times[(len(times)-1)*99/100]
	count := float64(c.Samples * c.Batch)
	result.BytesPerOp = float64(after.TotalAlloc-before.TotalAlloc) / count
	result.AllocsPerOp = float64(after.Mallocs-before.Mallocs) / count
	return result, nil
}
