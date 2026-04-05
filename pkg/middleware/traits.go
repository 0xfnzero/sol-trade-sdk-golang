package middleware

import (
	"fmt"
)

// Instruction represents a Solana instruction
type Instruction struct {
	ProgramID [32]byte
	Accounts  []AccountMeta
	Data      []byte
}

// AccountMeta represents account metadata for instructions
type AccountMeta struct {
	Pubkey     [32]byte
	IsSigner   bool
	IsWritable bool
}

// InstructionMiddleware interface for processing instructions
type InstructionMiddleware interface {
	// Name returns middleware name
	Name() string

	// ProcessProtocolInstructions processes protocol instructions
	ProcessProtocolInstructions(
		protocolInstructions []Instruction,
		protocolName string,
		isBuy bool,
	) ([]Instruction, error)

	// ProcessFullInstructions processes full instructions
	ProcessFullInstructions(
		fullInstructions []Instruction,
		protocolName string,
		isBuy bool,
	) ([]Instruction, error)

	// Clone creates a copy of the middleware
	Clone() InstructionMiddleware
}

// MiddlewareManager manages multiple middlewares
type MiddlewareManager struct {
	middlewares []InstructionMiddleware
}

// NewMiddlewareManager creates a new middleware manager
func NewMiddlewareManager() *MiddlewareManager {
	return &MiddlewareManager{
		middlewares: make([]InstructionMiddleware, 0),
	}
}

// AddMiddleware adds a middleware to the chain
func (m *MiddlewareManager) AddMiddleware(middleware InstructionMiddleware) *MiddlewareManager {
	m.middlewares = append(m.middlewares, middleware)
	return m
}

// ApplyMiddlewaresProcessProtocolInstructions applies all middlewares to protocol instructions
func (m *MiddlewareManager) ApplyMiddlewaresProcessProtocolInstructions(
	protocolInstructions []Instruction,
	protocolName string,
	isBuy bool,
) ([]Instruction, error) {
	result := protocolInstructions
	for _, middleware := range m.middlewares {
		var err error
		result, err = middleware.ProcessProtocolInstructions(result, protocolName, isBuy)
		if err != nil {
			return nil, err
		}
		if len(result) == 0 {
			break
		}
	}
	return result, nil
}

// ApplyMiddlewaresProcessFullInstructions applies all middlewares to full instructions
func (m *MiddlewareManager) ApplyMiddlewaresProcessFullInstructions(
	fullInstructions []Instruction,
	protocolName string,
	isBuy bool,
) ([]Instruction, error) {
	result := fullInstructions
	for _, middleware := range m.middlewares {
		var err error
		result, err = middleware.ProcessFullInstructions(result, protocolName, isBuy)
		if err != nil {
			return nil, err
		}
		if len(result) == 0 {
			break
		}
	}
	return result, nil
}

// WithCommonMiddlewares creates a manager with common middlewares
func WithCommonMiddlewares() *MiddlewareManager {
	return NewMiddlewareManager().AddMiddleware(&LoggingMiddleware{})
}

// Clone creates a deep copy of the manager
func (m *MiddlewareManager) Clone() *MiddlewareManager {
	cloned := NewMiddlewareManager()
	for _, mw := range m.middlewares {
		cloned.middlewares = append(cloned.middlewares, mw.Clone())
	}
	return cloned
}

// LoggingMiddleware logs instruction information
type LoggingMiddleware struct{}

// Name returns middleware name
func (l *LoggingMiddleware) Name() string {
	return "LoggingMiddleware"
}

// ProcessProtocolInstructions logs protocol instructions
func (l *LoggingMiddleware) ProcessProtocolInstructions(
	protocolInstructions []Instruction,
	protocolName string,
	isBuy bool,
) ([]Instruction, error) {
	fmt.Printf("-------------------[%s]-------------------\n", l.Name())
	fmt.Println("process_protocol_instructions")
	fmt.Printf("[%s] Instruction count: %d\n", l.Name(), len(protocolInstructions))
	fmt.Printf("[%s] Protocol name: %s\n\n", l.Name(), protocolName)
	fmt.Printf("[%s] Is buy: %v\n", l.Name(), isBuy)
	for i, instruction := range protocolInstructions {
		fmt.Printf("Instruction %d:\n", i+1)
		fmt.Printf("ProgramID: %x\n", instruction.ProgramID)
		fmt.Printf("Accounts: %d\n", len(instruction.Accounts))
		fmt.Printf("Data length: %d\n\n", len(instruction.Data))
	}
	return protocolInstructions, nil
}

// ProcessFullInstructions logs full instructions
func (l *LoggingMiddleware) ProcessFullInstructions(
	fullInstructions []Instruction,
	protocolName string,
	isBuy bool,
) ([]Instruction, error) {
	fmt.Printf("-------------------[%s]-------------------\n", l.Name())
	fmt.Println("process_full_instructions")
	fmt.Printf("[%s] Instruction count: %d\n", l.Name(), len(fullInstructions))
	fmt.Printf("[%s] Protocol name: %s\n\n", l.Name(), protocolName)
	fmt.Printf("[%s] Is buy: %v\n", l.Name(), isBuy)
	for i, instruction := range fullInstructions {
		fmt.Printf("Instruction %d:\n", i+1)
		fmt.Printf("ProgramID: %x\n", instruction.ProgramID)
		fmt.Printf("Accounts: %d\n", len(instruction.Accounts))
		fmt.Printf("Data length: %d\n\n", len(instruction.Data))
	}
	return fullInstructions, nil
}

// Clone creates a copy of the middleware
func (l *LoggingMiddleware) Clone() InstructionMiddleware {
	return &LoggingMiddleware{}
}
