package subscription

import (
	"errors"
	"sync"
)

type CacheReadinessStatus struct {
	State           string
	Generation      uint64
	ForkID          string
	MissingAccounts []string
	Reason          string
}
type CacheNotReadyError struct{ Status CacheReadinessStatus }

func (e *CacheNotReadyError) Error() string {
	return "CACHE_NOT_READY: cache unavailable or frozen continuity changed"
}

type SubscriptionReadiness struct {
	mu                  sync.RWMutex
	status              CacheReadinessStatus
	required, validated map[string]bool
}

func NewSubscriptionReadiness(forkID string, keys []string) (*SubscriptionReadiness, error) {
	if forkID == "" || len(keys) == 0 {
		return nil, errors.New("provide selected fork and required account identities")
	}
	r := &SubscriptionReadiness{status: CacheReadinessStatus{State: "initializing", ForkID: forkID}, required: map[string]bool{}, validated: map[string]bool{}}
	for _, k := range keys {
		if k == "" {
			return nil, errors.New("missing required account identity")
		}
		r.required[k] = true
	}
	return r, nil
}
func (r *SubscriptionReadiness) Status() CacheReadinessStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s := r.status
	s.MissingAccounts = nil
	for k := range r.required {
		if !r.validated[k] {
			s.MissingAccounts = append(s.MissingAccounts, k)
		}
	}
	return s
}
func (r *SubscriptionReadiness) Interrupt(reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status.Generation++
	r.status.State = "continuity-broken"
	r.status.Reason = reason
	r.validated = map[string]bool{}
}
func (r *SubscriptionReadiness) BeginRecovery(forkID string) (uint64, error) {
	if forkID == "" {
		return 0, errors.New("select a fork explicitly")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status.Generation++
	r.status.ForkID = forkID
	r.status.State = "recovering"
	r.status.Reason = ""
	r.validated = map[string]bool{}
	return r.status.Generation, nil
}
func (r *SubscriptionReadiness) RequireAccounts(keys []string) (uint64, error) {
	for _, k := range keys {
		if k == "" {
			return 0, errors.New("missing required account identity")
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	changed := false
	for _, k := range keys {
		if !r.required[k] {
			changed = true
			r.required[k] = true
		}
	}
	if changed {
		r.status.Generation++
		// New dependencies cannot restore a disconnected or fork-conflicted stream.
		if r.status.State != "continuity-broken" {
			r.status.State = "recovering"
		}
		r.validated = map[string]bool{}
	}
	return r.status.Generation, nil
}
func (r *SubscriptionReadiness) MarkValidated(keys []string, generation uint64, forkID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if generation != r.status.Generation || forkID != r.status.ForkID || r.status.State == "continuity-broken" {
		return false
	}
	for _, k := range keys {
		if r.required[k] {
			r.validated[k] = true
		}
	}
	if len(r.validated) == len(r.required) {
		r.status.State = "ready"
	}
	return r.status.State == "ready"
}
func (r *SubscriptionReadiness) Guard() (func() error, error) {
	r.mu.RLock()
	generation := r.status.Generation
	r.mu.RUnlock()
	check := func() error {
		r.mu.RLock()
		ready := r.status.State == "ready" && r.status.Generation == generation
		r.mu.RUnlock()
		if ready {
			return nil
		}
		// Copy dependency diagnostics only on failure; the healthy hot path is O(1).
		return &CacheNotReadyError{Status: r.Status()}
	}
	if err := check(); err != nil {
		return nil, err
	}
	return check, nil
}
