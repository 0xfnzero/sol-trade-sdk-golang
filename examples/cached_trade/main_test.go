package main

import (
	"encoding/json"
	"math"
	"testing"
)

func TestExactMaximumSlotAge(t *testing.T) {
	for _, document := range []string{`{"maximum_slot_age":18446744073709551615}`, `{"maximum_slot_age":"18446744073709551615"}`} {
		var request input
		if e := json.Unmarshal([]byte(document), &request); e != nil {
			t.Fatal(e)
		}
		n, e := jsonU64(request.MaximumSlotAge)
		if e != nil || n != math.MaxUint64 {
			t.Fatal(n, e)
		}
	}
}
func TestRejectInexactMaximumSlotAge(t *testing.T) {
	for _, raw := range []string{`"+1"`, `null`, `true`, `1.5`, `-1`, `"18446744073709551616"`} {
		if _, e := jsonU64(json.RawMessage(raw)); e == nil {
			t.Fatal("accepted", raw)
		}
	}
}
