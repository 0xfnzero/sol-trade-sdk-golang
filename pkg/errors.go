package soltradesdk

import "errors"

var (
	// ErrInvalidPrivateKey is returned when private key is invalid
	ErrInvalidPrivateKey = errors.New("invalid private key")
	// ErrInvalidAmount is returned when amount is zero or negative
	ErrInvalidAmount = errors.New("amount cannot be zero or negative")
	// ErrInvalidPercentage is returned when percentage is out of range
	ErrInvalidPercentage = errors.New("percentage must be between 1 and 100")
	// ErrMissingBlockhash is returned when blockhash is required but not provided
	ErrMissingBlockhash = errors.New("recent_blockhash or durable_nonce is required")
	// ErrInvalidProtocolParams is returned when protocol params don't match DEX type
	ErrInvalidProtocolParams = errors.New("invalid protocol params for DEX type")
	// ErrInsufficientBalance is returned when balance is insufficient
	ErrInsufficientBalance = errors.New("insufficient balance")
	// ErrTransactionFailed is returned when transaction fails
	ErrTransactionFailed = errors.New("transaction failed")
	// ErrRPCError is returned when RPC call fails
	ErrRPCError = errors.New("RPC error")
	// ErrTokenAccountNotFound is returned when token account doesn't exist
	ErrTokenAccountNotFound = errors.New("token account not found")
	// ErrPoolNotFound is returned when pool doesn't exist
	ErrPoolNotFound = errors.New("pool not found")
	// ErrUnsupportedDEX is returned when DEX type is not supported
	ErrUnsupportedDEX = errors.New("unsupported DEX type")
	// ErrUnsupportedToken is returned when token type is not supported for DEX
	ErrUnsupportedToken = errors.New("unsupported token type for this DEX")
)

// TradeError represents a trading error with details
type TradeError struct {
	Code    int
	Message string
	Cause   error
}

// Error implements the error interface
func (e *TradeError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Unwrap returns the underlying cause
func (e *TradeError) Unwrap() error {
	return e.Cause
}

// NewTradeError creates a new TradeError
func NewTradeError(code int, message string, cause error) *TradeError {
	return &TradeError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}
