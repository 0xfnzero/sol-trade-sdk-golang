package soltradesdk

import (
	"encoding/json"
	"fmt"
	"strings"
)

var solanaInstructionErrorCodes = map[string]int{
	"GenericError":              1,
	"InvalidArgument":           2,
	"InvalidInstructionData":    3,
	"InvalidAccountData":        4,
	"AccountDataTooSmall":       5,
	"InsufficientFunds":         6,
	"IncorrectProgramId":        7,
	"MissingRequiredSignature":  8,
	"AccountAlreadyInitialized": 9,
	"UninitializedAccount":      10,
}

// ParsedTransactionError is the Rust-parity representation of Solana meta.err.
type ParsedTransactionError struct {
	Code             int
	InstructionIndex *uint64
}

// ExtractHintsFromLogs extracts the same user-facing error log fragments as Rust.
func ExtractHintsFromLogs(logs []string) string {
	if len(logs) == 0 {
		return ""
	}

	parts := make([]string, 0)
	for _, log := range logs {
		if idx := strings.Index(log, "Error Message: "); idx >= 0 {
			parts = append(parts, strings.TrimSpace(strings.TrimSuffix(log[idx+15:], ".")))
			continue
		}
		if idx := strings.Index(log, "Program log: Error: "); idx >= 0 {
			parts = append(parts, strings.TrimSpace(strings.TrimSuffix(log[idx+20:], ".")))
		}
	}
	return strings.Join(parts, "; ")
}

// InstructionErrorCodeFromMetaErr maps Solana TransactionError JSON to Rust-compatible codes.
func InstructionErrorCodeFromMetaErr(errValue interface{}) ParsedTransactionError {
	if errValue == nil {
		return ParsedTransactionError{Code: 0}
	}

	normalized, ok := normalizeErrorMap(errValue)
	if !ok {
		return ParsedTransactionError{Code: 108}
	}

	rawInstructionError, ok := normalized["InstructionError"]
	if !ok {
		return ParsedTransactionError{Code: 108}
	}

	items, ok := rawInstructionError.([]interface{})
	if !ok || len(items) < 2 {
		return ParsedTransactionError{Code: 108}
	}

	instructionIndex := numberToUint64(items[0])
	detail := items[1]

	if detailMap, ok := normalizeErrorMap(detail); ok {
		if custom, ok := detailMap["Custom"]; ok {
			code := numberToUint64(custom)
			if code == nil {
				return ParsedTransactionError{Code: 999, InstructionIndex: instructionIndex}
			}
			return ParsedTransactionError{
				Code:             int(*code),
				InstructionIndex: instructionIndex,
			}
		}
	}

	if detailName, ok := detail.(string); ok {
		if code, ok := solanaInstructionErrorCodes[detailName]; ok {
			return ParsedTransactionError{Code: code, InstructionIndex: instructionIndex}
		}
	}

	return ParsedTransactionError{Code: 999, InstructionIndex: instructionIndex}
}

func normalizeErrorMap(value interface{}) (map[string]interface{}, bool) {
	switch v := value.(type) {
	case map[string]interface{}:
		return v, true
	case json.RawMessage:
		var out map[string]interface{}
		return out, json.Unmarshal(v, &out) == nil
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, false
		}
		var out map[string]interface{}
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, false
		}
		return out, true
	}
}

func numberToUint64(value interface{}) *uint64 {
	switch v := value.(type) {
	case uint64:
		return &v
	case uint32:
		out := uint64(v)
		return &out
	case uint:
		out := uint64(v)
		return &out
	case int:
		if v < 0 {
			return nil
		}
		out := uint64(v)
		return &out
	case int64:
		if v < 0 {
			return nil
		}
		out := uint64(v)
		return &out
	case float64:
		if v < 0 {
			return nil
		}
		out := uint64(v)
		return &out
	case json.Number:
		parsed, err := v.Int64()
		if err != nil || parsed < 0 {
			return nil
		}
		out := uint64(parsed)
		return &out
	default:
		return nil
	}
}

// FormatParsedTransactionError formats meta.err and logs into a TradeError.
func FormatParsedTransactionError(errValue interface{}, logs []string) *TradeError {
	parsed := InstructionErrorCodeFromMetaErr(errValue)
	hints := ExtractHintsFromLogs(logs)
	message := fmt.Sprintf("%v", errValue)
	if hints != "" {
		message = fmt.Sprintf("%s %s", message, hints)
	}
	return &TradeError{
		Code:             parsed.Code,
		Message:          message,
		InstructionIndex: parsed.InstructionIndex,
	}
}
