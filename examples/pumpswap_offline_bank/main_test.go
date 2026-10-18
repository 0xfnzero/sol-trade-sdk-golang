package main

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPumpSwapDeployedProgramBank(t *testing.T) {
	root, e := filepath.Abs("../..")
	if e != nil {
		t.Fatal(e)
	}
	engine, cleanup, e := buildEngine(root)
	if e != nil {
		t.Fatal(e)
	}
	defer cleanup()
	for _, buy := range []bool{true, false} {
		for _, direct := range []bool{true, false} {
			for _, alt := range []bool{false, true} {
				if !direct && !alt {
					continue
				}
				for _, reject := range []bool{false, true} {
					var baseline bankResult
					for iteration := 0; iteration < 2; iteration++ {
						result, e := runCase(root, engine, buy, direct, alt, reject)
						if e != nil {
							t.Fatalf("buy=%t ALT=%t reject=%t: %v", buy, alt, reject, e)
						}
						if iteration == 0 {
							baseline = result
						} else if !reflect.DeepEqual(result, baseline) {
							t.Fatal("fresh bank result is not deterministic")
						}
					}
					encoded, _ := json.Marshal(struct {
						Buy, ALT, Reject bool
						CU               uint64
					}{buy, alt, reject, baseline.Units})
					t.Log(string(encoded))
				}
			}
		}
	}
}
