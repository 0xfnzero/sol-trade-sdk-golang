package middleware

import (
	"fmt"
	"time"
)

// TimerMiddleware measures execution time of instruction processing
type TimerMiddleware struct {
	enabled bool
}

// NewTimerMiddleware creates a new timer middleware
func NewTimerMiddleware() *TimerMiddleware {
	return &TimerMiddleware{enabled: true}
}

// Name returns middleware name
func (t *TimerMiddleware) Name() string {
	return "TimerMiddleware"
}

// ProcessProtocolInstructions measures protocol instruction processing time
func (t *TimerMiddleware) ProcessProtocolInstructions(
	protocolInstructions []Instruction,
	protocolName string,
	isBuy bool,
) ([]Instruction, error) {
	if !t.enabled {
		return protocolInstructions, nil
	}

	start := time.Now()
	fmt.Printf("[%s] Processing protocol instructions for %s (buy: %v)\n", t.Name(), protocolName, isBuy)
	fmt.Printf("[%s] Processing time: %v\n", t.Name(), time.Since(start))
	return protocolInstructions, nil
}

// ProcessFullInstructions measures full instruction processing time
func (t *TimerMiddleware) ProcessFullInstructions(
	fullInstructions []Instruction,
	protocolName string,
	isBuy bool,
) ([]Instruction, error) {
	if !t.enabled {
		return fullInstructions, nil
	}

	start := time.Now()
	fmt.Printf("[%s] Processing full instructions for %s (buy: %v)\n", t.Name(), protocolName, isBuy)
	fmt.Printf("[%s] Processing time: %v\n", t.Name(), time.Since(start))
	return fullInstructions, nil
}

// Clone creates a copy of the middleware
func (t *TimerMiddleware) Clone() InstructionMiddleware {
	return &TimerMiddleware{enabled: t.enabled}
}

// ValidationMiddleware validates instructions before processing
type ValidationMiddleware struct {
	MaxInstructions int
	MaxDataSize     int
}

// NewValidationMiddleware creates a new validation middleware
func NewValidationMiddleware(maxInstructions, maxDataSize int) *ValidationMiddleware {
	return &ValidationMiddleware{
		MaxInstructions: maxInstructions,
		MaxDataSize:     maxDataSize,
	}
}

// Name returns middleware name
func (v *ValidationMiddleware) Name() string {
	return "ValidationMiddleware"
}

// ProcessProtocolInstructions validates protocol instructions
func (v *ValidationMiddleware) ProcessProtocolInstructions(
	protocolInstructions []Instruction,
	protocolName string,
	isBuy bool,
) ([]Instruction, error) {
	if err := v.validate(protocolInstructions); err != nil {
		return nil, fmt.Errorf("[%s] validation failed: %w", v.Name(), err)
	}
	return protocolInstructions, nil
}

// ProcessFullInstructions validates full instructions
func (v *ValidationMiddleware) ProcessFullInstructions(
	fullInstructions []Instruction,
	protocolName string,
	isBuy bool,
) ([]Instruction, error) {
	if err := v.validate(fullInstructions); err != nil {
		return nil, fmt.Errorf("[%s] validation failed: %w", v.Name(), err)
	}
	return fullInstructions, nil
}

// Clone creates a copy of the middleware
func (v *ValidationMiddleware) Clone() InstructionMiddleware {
	return &ValidationMiddleware{
		MaxInstructions: v.MaxInstructions,
		MaxDataSize:     v.MaxDataSize,
	}
}

// validate checks if instructions meet validation criteria
func (v *ValidationMiddleware) validate(instructions []Instruction) error {
	if v.MaxInstructions > 0 && len(instructions) > v.MaxInstructions {
		return fmt.Errorf("too many instructions: %d > %d", len(instructions), v.MaxInstructions)
	}

	for i, instr := range instructions {
		if v.MaxDataSize > 0 && len(instr.Data) > v.MaxDataSize {
			return fmt.Errorf("instruction %d data too large: %d > %d", i, len(instr.Data), v.MaxDataSize)
		}
	}

	return nil
}

// FilterMiddleware filters instructions based on program ID
type FilterMiddleware struct {
	allowedPrograms map[[32]byte]bool
	filterMode      FilterMode
}

// FilterMode represents filter behavior
type FilterMode int

const (
	// FilterModeAllow allows only specified programs
	FilterModeAllow FilterMode = iota
	// FilterModeBlock blocks specified programs
	FilterModeBlock
)

// NewFilterMiddleware creates a new filter middleware
func NewFilterMiddleware(programs [][32]byte, mode FilterMode) *FilterMiddleware {
	allowed := make(map[[32]byte]bool)
	for _, p := range programs {
		allowed[p] = true
	}
	return &FilterMiddleware{
		allowedPrograms: allowed,
		filterMode:      mode,
	}
}

// Name returns middleware name
func (f *FilterMiddleware) Name() string {
	return "FilterMiddleware"
}

// ProcessProtocolInstructions filters protocol instructions
func (f *FilterMiddleware) ProcessProtocolInstructions(
	protocolInstructions []Instruction,
	protocolName string,
	isBuy bool,
) ([]Instruction, error) {
	return f.filter(protocolInstructions), nil
}

// ProcessFullInstructions filters full instructions
func (f *FilterMiddleware) ProcessFullInstructions(
	fullInstructions []Instruction,
	protocolName string,
	isBuy bool,
) ([]Instruction, error) {
	return f.filter(fullInstructions), nil
}

// Clone creates a copy of the middleware
func (f *FilterMiddleware) Clone() InstructionMiddleware {
	programs := make([][32]byte, 0, len(f.allowedPrograms))
	for p := range f.allowedPrograms {
		programs = append(programs, p)
	}
	return NewFilterMiddleware(programs, f.filterMode)
}

// filter applies the filter to instructions
func (f *FilterMiddleware) filter(instructions []Instruction) []Instruction {
	result := make([]Instruction, 0)
	for _, instr := range instructions {
		allowed := f.allowedPrograms[instr.ProgramID]
		if f.filterMode == FilterModeAllow && allowed {
			result = append(result, instr)
		} else if f.filterMode == FilterModeBlock && !allowed {
			result = append(result, instr)
		}
	}
	return result
}

// MetricsMiddleware collects metrics about instruction processing
type MetricsMiddleware struct {
	instructionCounts map[string]int64
	totalInstructions int64
	totalDataSize     int64
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware() *MetricsMiddleware {
	return &MetricsMiddleware{
		instructionCounts: make(map[string]int64),
	}
}

// Name returns middleware name
func (m *MetricsMiddleware) Name() string {
	return "MetricsMiddleware"
}

// ProcessProtocolInstructions collects metrics for protocol instructions
func (m *MetricsMiddleware) ProcessProtocolInstructions(
	protocolInstructions []Instruction,
	protocolName string,
	isBuy bool,
) ([]Instruction, error) {
	m.record(protocolName, protocolInstructions)
	return protocolInstructions, nil
}

// ProcessFullInstructions collects metrics for full instructions
func (m *MetricsMiddleware) ProcessFullInstructions(
	fullInstructions []Instruction,
	protocolName string,
	isBuy bool,
) ([]Instruction, error) {
	m.record(protocolName, fullInstructions)
	return fullInstructions, nil
}

// Clone creates a copy of the middleware
func (m *MetricsMiddleware) Clone() InstructionMiddleware {
	newCounts := make(map[string]int64)
	for k, v := range m.instructionCounts {
		newCounts[k] = v
	}
	return &MetricsMiddleware{
		instructionCounts: newCounts,
		totalInstructions: m.totalInstructions,
		totalDataSize:     m.totalDataSize,
	}
}

// record updates metrics
func (m *MetricsMiddleware) record(protocolName string, instructions []Instruction) {
	m.instructionCounts[protocolName] += int64(len(instructions))
	m.totalInstructions += int64(len(instructions))

	for _, instr := range instructions {
		m.totalDataSize += int64(len(instr.Data))
	}
}

// GetMetrics returns collected metrics
func (m *MetricsMiddleware) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"instruction_counts": m.instructionCounts,
		"total_instructions": m.totalInstructions,
		"total_data_size":    m.totalDataSize,
	}
}

// WithStandardMiddlewares creates a manager with standard middlewares
func WithStandardMiddlewares() *MiddlewareManager {
	return NewMiddlewareManager().
		AddMiddleware(NewValidationMiddleware(100, 10000)).
		AddMiddleware(&LoggingMiddleware{}).
		AddMiddleware(NewTimerMiddleware())
}

// WithAllBuiltinMiddlewares creates a manager with all builtin middlewares
func WithAllBuiltinMiddlewares() *MiddlewareManager {
	return NewMiddlewareManager().
		AddMiddleware(NewValidationMiddleware(100, 10000)).
		AddMiddleware(&LoggingMiddleware{}).
		AddMiddleware(NewTimerMiddleware()).
		AddMiddleware(NewMetricsMiddleware())
}
