package middleware

import (
	"github.com/gagliardetto/solana-go"
)

// Middleware defines the interface for instruction middleware
type Middleware interface {
	// Process is called before transaction execution
	// It can modify, add, or remove instructions
	Process(instructions []solana.Instruction, context *MiddlewareContext) ([]solana.Instruction, error)

	// Name returns the middleware name for logging
	Name() string
}

// MiddlewareContext provides context for middleware execution
type MiddlewareContext struct {
	TradeType     string
	InputMint     solana.PublicKey
	OutputMint    solana.PublicKey
	InputAmount   uint64
	Payer         solana.PublicKey
	AdditionalData map[string]interface{}
}

// MiddlewareManager manages a chain of middleware
type MiddlewareManager struct {
	middlewares []Middleware
}

// NewMiddlewareManager creates a new middleware manager
func NewMiddlewareManager() *MiddlewareManager {
	return &MiddlewareManager{
		middlewares: make([]Middleware, 0),
	}
}

// AddMiddleware adds a middleware to the chain
func (m *MiddlewareManager) AddMiddleware(mw Middleware) *MiddlewareManager {
	m.middlewares = append(m.middlewares, mw)
	return m
}

// Process runs all middleware in order
func (m *MiddlewareManager) Process(instructions []solana.Instruction, context *MiddlewareContext) ([]solana.Instruction, error) {
	var err error
	result := instructions

	for _, mw := range m.middlewares {
		result, err = mw.Process(result, context)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// LoggingMiddleware logs all instructions before execution
type LoggingMiddleware struct{}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware() *LoggingMiddleware {
	return &LoggingMiddleware{}
}

// Process implements Middleware
func (m *LoggingMiddleware) Process(instructions []solana.Instruction, context *MiddlewareContext) ([]solana.Instruction, error) {
	// Log instruction details (simplified)
	return instructions, nil
}

// Name implements Middleware
func (m *LoggingMiddleware) Name() string {
	return "LoggingMiddleware"
}

// ValidationMiddleware validates instructions before execution
type ValidationMiddleware struct{}

// NewValidationMiddleware creates a new validation middleware
func NewValidationMiddleware() *ValidationMiddleware {
	return &ValidationMiddleware{}
}

// Process implements Middleware
func (m *ValidationMiddleware) Process(instructions []solana.Instruction, context *MiddlewareContext) ([]solana.Instruction, error) {
	// Validate instructions (simplified)
	return instructions, nil
}

// Name implements Middleware
func (m *ValidationMiddleware) Name() string {
	return "ValidationMiddleware"
}
