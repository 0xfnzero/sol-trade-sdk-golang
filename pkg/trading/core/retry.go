package core

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// RetryStrategy defines the interface for retry strategies
type RetryStrategy interface {
	// NextDelay returns the delay for the next retry attempt
	NextDelay(attempt int) time.Duration
	// ShouldRetry returns true if another retry should be attempted
	ShouldRetry(attempt int, err error) bool
	// Reset resets the strategy state
	Reset()
}

// RetryConfig represents configuration for retry handling
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int
	// InitialDelay is the initial delay between retries
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration
	// Multiplier is the exponential backoff multiplier
	Multiplier float64
	// Jitter adds random jitter to delays (0.0 to 1.0)
	Jitter float64
	// RetryableErrors is a list of error types that should trigger retry
	RetryableErrors []error
	// NonRetryableErrors is a list of error types that should not trigger retry
	NonRetryableErrors []error
	// Timeout is the maximum total time for all retries
	Timeout time.Duration
	// Strategy is the retry strategy to use (nil for default exponential)
	Strategy RetryStrategy
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:         3,
		InitialDelay:       100 * time.Millisecond,
		MaxDelay:           30 * time.Second,
		Multiplier:         2.0,
		Jitter:             0.1,
		Timeout:            60 * time.Second,
		RetryableErrors:    make([]error, 0),
		NonRetryableErrors: make([]error, 0),
	}
}

// Validate validates the retry configuration
func (c *RetryConfig) Validate() error {
	if c.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}
	if c.InitialDelay < 0 {
		return fmt.Errorf("initial delay cannot be negative")
	}
	if c.MaxDelay < c.InitialDelay {
		return fmt.Errorf("max delay cannot be less than initial delay")
	}
	if c.Multiplier < 1.0 {
		return fmt.Errorf("multiplier must be at least 1.0")
	}
	if c.Jitter < 0 || c.Jitter > 1.0 {
		return fmt.Errorf("jitter must be between 0.0 and 1.0")
	}
	return nil
}

// ExponentialBackoff implements exponential backoff retry strategy
type ExponentialBackoff struct {
	config  *RetryConfig
	attempt int
	mu      sync.Mutex
}

// NewExponentialBackoff creates a new exponential backoff strategy
func NewExponentialBackoff(config *RetryConfig) *ExponentialBackoff {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &ExponentialBackoff{
		config: config,
	}
}

// NextDelay returns the delay for the next retry attempt
func (e *ExponentialBackoff) NextDelay(attempt int) time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()

	if attempt <= 0 {
		return 0
	}

	// Calculate exponential delay
	delay := float64(e.config.InitialDelay) * math.Pow(e.config.Multiplier, float64(attempt-1))

	// Apply max delay cap
	if delay > float64(e.config.MaxDelay) {
		delay = float64(e.config.MaxDelay)
	}

	// Apply jitter
	if e.config.Jitter > 0 {
		jitter := delay * e.config.Jitter * (2*rand.Float64() - 1)
		delay += jitter
	}

	return time.Duration(delay)
}

// ShouldRetry returns true if another retry should be attempted
func (e *ExponentialBackoff) ShouldRetry(attempt int, err error) bool {
	if err == nil {
		return false
	}
	if attempt >= e.config.MaxRetries {
		return false
	}

	// Check non-retryable errors
	for _, nonRetryable := range e.config.NonRetryableErrors {
		if err == nonRetryable {
			return false
		}
	}

	// If specific retryable errors are defined, check them
	if len(e.config.RetryableErrors) > 0 {
		for _, retryable := range e.config.RetryableErrors {
			if err == retryable {
				return true
			}
		}
		return false
	}

	return true
}

// Reset resets the strategy state
func (e *ExponentialBackoff) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.attempt = 0
}

// LinearBackoff implements linear backoff retry strategy
type LinearBackoff struct {
	config *RetryConfig
	mu     sync.Mutex
}

// NewLinearBackoff creates a new linear backoff strategy
func NewLinearBackoff(config *RetryConfig) *LinearBackoff {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &LinearBackoff{
		config: config,
	}
}

// NextDelay returns the delay for the next retry attempt
func (l *LinearBackoff) NextDelay(attempt int) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	if attempt <= 0 {
		return 0
	}

	delay := time.Duration(attempt) * l.config.InitialDelay
	if delay > l.config.MaxDelay {
		delay = l.config.MaxDelay
	}

	// Apply jitter
	if l.config.Jitter > 0 {
		jitter := float64(delay) * l.config.Jitter * (2*rand.Float64() - 1)
		delay = time.Duration(float64(delay) + jitter)
	}

	return delay
}

// ShouldRetry returns true if another retry should be attempted
func (l *LinearBackoff) ShouldRetry(attempt int, err error) bool {
	if err == nil {
		return false
	}
	if attempt >= l.config.MaxRetries {
		return false
	}

	for _, nonRetryable := range l.config.NonRetryableErrors {
		if err == nonRetryable {
			return false
		}
	}

	if len(l.config.RetryableErrors) > 0 {
		for _, retryable := range l.config.RetryableErrors {
			if err == retryable {
				return true
			}
		}
		return false
	}

	return true
}

// Reset resets the strategy state
func (l *LinearBackoff) Reset() {}

// FixedDelay implements fixed delay retry strategy
type FixedDelay struct {
	config *RetryConfig
}

// NewFixedDelay creates a new fixed delay strategy
func NewFixedDelay(config *RetryConfig) *FixedDelay {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &FixedDelay{
		config: config,
	}
}

// NextDelay returns the delay for the next retry attempt
func (f *FixedDelay) NextDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	delay := f.config.InitialDelay
	if f.config.Jitter > 0 {
		jitter := float64(delay) * f.config.Jitter * (2*rand.Float64() - 1)
		delay = time.Duration(float64(delay) + jitter)
	}
	return delay
}

// ShouldRetry returns true if another retry should be attempted
func (f *FixedDelay) ShouldRetry(attempt int, err error) bool {
	if err == nil {
		return false
	}
	if attempt >= f.config.MaxRetries {
		return false
	}

	for _, nonRetryable := range f.config.NonRetryableErrors {
		if err == nonRetryable {
			return false
		}
	}

	if len(f.config.RetryableErrors) > 0 {
		for _, retryable := range f.config.RetryableErrors {
			if err == retryable {
				return true
			}
		}
		return false
	}

	return true
}

// Reset resets the strategy state
func (f *FixedDelay) Reset() {}

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	// CircuitStateClosed allows requests through
	CircuitStateClosed CircuitState = iota
	// CircuitStateOpen blocks all requests
	CircuitStateOpen
	// CircuitStateHalfOpen allows test requests
	CircuitStateHalfOpen
)

func (c CircuitState) String() string {
	return [...]string{"Closed", "Open", "HalfOpen"}[c]
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	// FailureThreshold is the number of failures before opening
	FailureThreshold int
	// SuccessThreshold is the number of successes in half-open to close
	SuccessThreshold int
	// Timeout is the time before transitioning from open to half-open
	Timeout time.Duration

	state           CircuitState
	failures        int
	successes       int
	lastFailureTime time.Time
	mu              sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		FailureThreshold: failureThreshold,
		SuccessThreshold: successThreshold,
		Timeout:          timeout,
		state:            CircuitStateClosed,
	}
}

// DefaultCircuitBreaker returns a circuit breaker with default settings
func DefaultCircuitBreaker() *CircuitBreaker {
	return NewCircuitBreaker(5, 3, 30*time.Second)
}

// Allow returns true if the request should be allowed
func (c *CircuitBreaker) Allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.state {
	case CircuitStateClosed:
		return true
	case CircuitStateOpen:
		if time.Since(c.lastFailureTime) > c.Timeout {
			c.state = CircuitStateHalfOpen
			c.failures = 0
			c.successes = 0
			return true
		}
		return false
	case CircuitStateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful request
func (c *CircuitBreaker) RecordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.state {
	case CircuitStateClosed:
		c.failures = 0
	case CircuitStateHalfOpen:
		c.successes++
		if c.successes >= c.SuccessThreshold {
			c.state = CircuitStateClosed
			c.failures = 0
			c.successes = 0
		}
	}
}

// RecordFailure records a failed request
func (c *CircuitBreaker) RecordFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastFailureTime = time.Now()

	switch c.state {
	case CircuitStateClosed:
		c.failures++
		if c.failures >= c.FailureThreshold {
			c.state = CircuitStateOpen
		}
	case CircuitStateHalfOpen:
		c.state = CircuitStateOpen
		c.failures = 0
		c.successes = 0
	}
}

// State returns the current circuit state
func (c *CircuitBreaker) State() CircuitState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// Stats returns circuit breaker statistics
func (c *CircuitBreaker) Stats() CircuitStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CircuitStats{
		State:           c.state,
		Failures:        c.failures,
		Successes:       c.successes,
		LastFailureTime: c.lastFailureTime,
	}
}

// Reset resets the circuit breaker to closed state
func (c *CircuitBreaker) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = CircuitStateClosed
	c.failures = 0
	c.successes = 0
}

// CircuitStats represents circuit breaker statistics
type CircuitStats struct {
	State           CircuitState
	Failures        int
	Successes       int
	LastFailureTime time.Time
}

// RetryHandler handles retry logic with circuit breaker
type RetryHandler struct {
	config         *RetryConfig
	strategy       RetryStrategy
	circuitBreaker *CircuitBreaker
	totalAttempts  int64
	successCount   int64
	failureCount   int64
	retryCount     int64
	mu             sync.RWMutex
}

// NewRetryHandler creates a new retry handler
func NewRetryHandler(config *RetryConfig, circuitBreaker *CircuitBreaker) (*RetryHandler, error) {
	if config == nil {
		config = DefaultRetryConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid retry config: %w", err)
	}

	strategy := config.Strategy
	if strategy == nil {
		strategy = NewExponentialBackoff(config)
	}

	return &RetryHandler{
		config:         config,
		strategy:       strategy,
		circuitBreaker: circuitBreaker,
	}, nil
}

// Execute executes a function with retry logic
func (r *RetryHandler) Execute(ctx context.Context, fn func() error) error {
	// Check circuit breaker
	if r.circuitBreaker != nil && !r.circuitBreaker.Allow() {
		return fmt.Errorf("circuit breaker is open")
	}

	atomic.AddInt64(&r.totalAttempts, 1)

	// Create timeout context if needed
	if r.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.config.Timeout)
		defer cancel()
	}

	var lastErr error
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}

		// Execute the function
		err := fn()
		if err == nil {
			// Success
			atomic.AddInt64(&r.successCount, 1)
			if r.circuitBreaker != nil {
				r.circuitBreaker.RecordSuccess()
			}
			r.strategy.Reset()
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !r.strategy.ShouldRetry(attempt, err) {
			break
		}

		atomic.AddInt64(&r.retryCount, 1)

		// Wait before retry
		if attempt < r.config.MaxRetries {
			delay := r.strategy.NextDelay(attempt + 1)
			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return fmt.Errorf("retry cancelled during delay: %w", ctx.Err())
				}
			}
		}
	}

	// All retries exhausted
	atomic.AddInt64(&r.failureCount, 1)
	if r.circuitBreaker != nil {
		r.circuitBreaker.RecordFailure()
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// ExecuteWithResult executes a function that returns a result with retry logic
func (r *RetryHandler) ExecuteWithResult(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	// Check circuit breaker
	if r.circuitBreaker != nil && !r.circuitBreaker.Allow() {
		return nil, fmt.Errorf("circuit breaker is open")
	}

	atomic.AddInt64(&r.totalAttempts, 1)

	// Create timeout context if needed
	if r.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.config.Timeout)
		defer cancel()
	}

	var lastErr error
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}

		// Execute the function
		result, err := fn()
		if err == nil {
			// Success
			atomic.AddInt64(&r.successCount, 1)
			if r.circuitBreaker != nil {
				r.circuitBreaker.RecordSuccess()
			}
			r.strategy.Reset()
			return result, nil
		}

		lastErr = err

		// Check if we should retry
		if !r.strategy.ShouldRetry(attempt, err) {
			break
		}

		atomic.AddInt64(&r.retryCount, 1)

		// Wait before retry
		if attempt < r.config.MaxRetries {
			delay := r.strategy.NextDelay(attempt + 1)
			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return nil, fmt.Errorf("retry cancelled during delay: %w", ctx.Err())
				}
			}
		}
	}

	// All retries exhausted
	atomic.AddInt64(&r.failureCount, 1)
	if r.circuitBreaker != nil {
		r.circuitBreaker.RecordFailure()
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// GetStats returns retry handler statistics
func (r *RetryHandler) GetStats() RetryStats {
	return RetryStats{
		TotalAttempts: atomic.LoadInt64(&r.totalAttempts),
		SuccessCount:  atomic.LoadInt64(&r.successCount),
		FailureCount:  atomic.LoadInt64(&r.failureCount),
		RetryCount:    atomic.LoadInt64(&r.retryCount),
	}
}

// Reset resets the retry handler statistics
func (r *RetryHandler) Reset() {
	atomic.StoreInt64(&r.totalAttempts, 0)
	atomic.StoreInt64(&r.successCount, 0)
	atomic.StoreInt64(&r.failureCount, 0)
	atomic.StoreInt64(&r.retryCount, 0)
	r.strategy.Reset()
}

// RetryStats represents retry handler statistics
type RetryStats struct {
	TotalAttempts int64
	SuccessCount  int64
	FailureCount  int64
	RetryCount    int64
}

// SuccessRate returns the success rate as a percentage
func (s RetryStats) SuccessRate() float64 {
	if s.TotalAttempts == 0 {
		return 0
	}
	return float64(s.SuccessCount) / float64(s.TotalAttempts) * 100
}

// RetryableError wraps an error to mark it as retryable
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("retryable: %v", e.Err)
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true if the error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var retryable *RetryableError
	return err == retryable || fmt.Sprintf("%T", err) == "*core.RetryableError"
}

// NonRetryableError wraps an error to mark it as non-retryable
type NonRetryableError struct {
	Err error
}

func (e *NonRetryableError) Error() string {
	return fmt.Sprintf("non-retryable: %v", e.Err)
}

func (e *NonRetryableError) Unwrap() error {
	return e.Err
}

// IsNonRetryable returns true if the error is non-retryable
func IsNonRetryable(err error) bool {
	if err == nil {
		return false
	}
	var nonRetryable *NonRetryableError
	return err == nonRetryable || fmt.Sprintf("%T", err) == "*core.NonRetryableError"
}

// WithRetryable wraps an error as retryable
func WithRetryable(err error) error {
	if err == nil {
		return nil
	}
	return &RetryableError{Err: err}
}

// WithNonRetryable wraps an error as non-retryable
func WithNonRetryable(err error) error {
	if err == nil {
		return nil
	}
	return &NonRetryableError{Err: err}
}
