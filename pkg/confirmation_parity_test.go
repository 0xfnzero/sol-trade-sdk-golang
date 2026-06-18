package soltradesdk

import "testing"

func TestInstructionErrorCodeFromMetaErr(t *testing.T) {
	custom := InstructionErrorCodeFromMetaErr(map[string]interface{}{
		"InstructionError": []interface{}{float64(2), map[string]interface{}{"Custom": float64(6001)}},
	})
	if custom.Code != 6001 || custom.InstructionIndex == nil || *custom.InstructionIndex != 2 {
		t.Fatalf("custom parse = %+v", custom)
	}

	builtin := InstructionErrorCodeFromMetaErr(map[string]interface{}{
		"InstructionError": []interface{}{float64(1), "InvalidInstructionData"},
	})
	if builtin.Code != 3 || builtin.InstructionIndex == nil || *builtin.InstructionIndex != 1 {
		t.Fatalf("builtin parse = %+v", builtin)
	}

	unknown := InstructionErrorCodeFromMetaErr(map[string]interface{}{
		"InstructionError": []interface{}{float64(4), "ComputationalBudgetExceeded"},
	})
	if unknown.Code != 999 || unknown.InstructionIndex == nil || *unknown.InstructionIndex != 4 {
		t.Fatalf("unknown parse = %+v", unknown)
	}

	nonInstruction := InstructionErrorCodeFromMetaErr("BlockhashNotFound")
	if nonInstruction.Code != 108 || nonInstruction.InstructionIndex != nil {
		t.Fatalf("non-instruction parse = %+v", nonInstruction)
	}
}

func TestExtractHintsFromLogs(t *testing.T) {
	got := ExtractHintsFromLogs([]string{
		"Program log: Error: slippage.",
		"x Error Message: user rejected.",
	})
	if got != "slippage; user rejected" {
		t.Fatalf("hints = %q", got)
	}
}
