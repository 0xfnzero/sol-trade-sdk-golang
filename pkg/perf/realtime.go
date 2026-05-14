package perf

import (
	"runtime"
	"syscall"
	"time"
)

// ThreadPriority represents thread priority levels
type ThreadPriority int

const (
	PriorityIdle ThreadPriority = iota
	PriorityLowest
	PriorityBelowNormal
	PriorityNormal
	PriorityAboveNormal
	PriorityHighest
	PriorityTimeCritical
)

// RealtimeConfig configuration for real-time tuning
type RealtimeConfig struct {
	EnableRealtimeSched bool
	Priority            ThreadPriority
	NiceValue           int
	CPUAffinity         []int
}

// DefaultRealtimeConfig returns default config
func DefaultRealtimeConfig() *RealtimeConfig {
	return &RealtimeConfig{
		EnableRealtimeSched: true,
		Priority:            PriorityAboveNormal,
		NiceValue:           -10,
	}
}

// RealtimeTuner provides real-time performance tuning
type RealtimeTuner struct {
	config *RealtimeConfig
}

// NewRealtimeTuner creates real-time tuner
func NewRealtimeTuner(config *RealtimeConfig) *RealtimeTuner {
	if config == nil {
		config = DefaultRealtimeConfig()
	}
	return &RealtimeTuner{config: config}
}

// TuneProcess tunes the current process for real-time
func (r *RealtimeTuner) TuneProcess() error {
	if runtime.GOOS != "linux" {
		return nil // Only supported on Linux
	}

	// Set nice value
	if r.config.NiceValue != 0 {
		syscall.Setpriority(syscall.PRIO_PROCESS, 0, r.config.NiceValue)
	}

	// Set scheduler if requested
	if r.config.EnableRealtimeSched {
		// SCHED_FIFO = 1
		// This requires root privileges
		// In real implementation, use syscall.SchedSetscheduler
	}

	return nil
}

// TuneThread tunes the current thread
func (r *RealtimeTuner) TuneThread() error {
	// Lock to OS thread
	runtime.LockOSThread()

	// Set thread priority if on Linux
	if runtime.GOOS == "linux" {
		// Use setpriority for thread
		if r.config.NiceValue != 0 {
			syscall.Setpriority(syscall.PRIO_PROCESS, 0, r.config.NiceValue)
		}
	}

	return nil
}

// Reset resets process/thread tuning
func (r *RealtimeTuner) Reset() error {
	runtime.UnlockOSThread()
	return nil
}

// ThreadPriorityManager manages thread priorities
type ThreadPriorityManager struct {
	priorities map[int]ThreadPriority
}

// NewThreadPriorityManager creates priority manager
func NewThreadPriorityManager() *ThreadPriorityManager {
	return &ThreadPriorityManager{
		priorities: make(map[int]ThreadPriority),
	}
}

// SetPriority sets priority for current thread
func (m *ThreadPriorityManager) SetPriority(priority ThreadPriority) error {
	if runtime.GOOS != "linux" {
		return nil
	}

	niceValue := 0
	switch priority {
	case PriorityIdle:
		niceValue = 19
	case PriorityLowest:
		niceValue = 10
	case PriorityBelowNormal:
		niceValue = 5
	case PriorityNormal:
		niceValue = 0
	case PriorityAboveNormal:
		niceValue = -5
	case PriorityHighest:
		niceValue = -10
	case PriorityTimeCritical:
		niceValue = -20
	}

	return syscall.Setpriority(syscall.PRIO_PROCESS, 0, niceValue)
}

// GetPriority gets current thread priority
func (m *ThreadPriorityManager) GetPriority() (ThreadPriority, error) {
	if runtime.GOOS != "linux" {
		return PriorityNormal, nil
	}

	nice, err := syscall.Getpriority(syscall.PRIO_PROCESS, 0)
	if err != nil {
		return PriorityNormal, err
	}

	switch {
	case nice >= 19:
		return PriorityIdle, nil
	case nice >= 10:
		return PriorityLowest, nil
	case nice >= 5:
		return PriorityBelowNormal, nil
	case nice >= 0:
		return PriorityNormal, nil
	case nice >= -5:
		return PriorityAboveNormal, nil
	case nice >= -10:
		return PriorityHighest, nil
	default:
		return PriorityTimeCritical, nil
	}
}

// BusyWait spins for a duration
func BusyWait(duration time.Duration) {
	start := time.Now()
	for time.Since(start) < duration {
		// Busy spin
	}
}

// Yield yields the processor
func Yield() {
	runtime.Gosched()
}
