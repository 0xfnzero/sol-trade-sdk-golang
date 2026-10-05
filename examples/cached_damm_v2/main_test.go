package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestExplicitFactoryWires(t *testing.T) {
	for _, asset := range []string{"sol", "wsol"} {
		t.Run(asset, func(t *testing.T) {
			d, e := os.ReadFile("../fixtures/cached_damm_v2_" + asset + "_buy_20261004.json")
			if e != nil {
				t.Fatal(e)
			}
			var v input
			if e = json.Unmarshal(d, &v); e != nil {
				t.Fatal(e)
			}
			wire, e := build(v)
			if e != nil {
				t.Fatal(e)
			}
			var expected struct {
				Expected struct {
					Hash string `json:"wire_sha256"`
				}
			}
			json.Unmarshal(d, &expected)
			if fmt.Sprintf("%x", sha256.Sum256(wire)) != expected.Expected.Hash {
				t.Fatal("wire mismatch")
			}
		})
	}
}

func TestIndependentSellPreparation(t *testing.T) {
	d, e := os.ReadFile("../fixtures/cached_damm_v2_wsol_buy_20261004.json")
	if e != nil {
		t.Fatal(e)
	}
	var v input
	if e = json.Unmarshal(d, &v); e != nil {
		t.Fatal(e)
	}
	v.TradeType = "Sell"
	v.Legs[0].Input, v.Legs[0].Output = v.Legs[0].Output, v.Legs[0].Input
	if _, e = build(v); e != nil {
		t.Fatal(e)
	}
	v.NativeOutput = true
	v.RentLamports = "2039280"
	v.TemporaryWsolSeed = "damm-test-sell"
	if _, e = build(v); e != nil {
		t.Fatal(e)
	}
}
