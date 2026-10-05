package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestVerifiedWire(t *testing.T) {
	d, e := os.ReadFile("../fixtures/damm_v2_buy_mainnet_20261004.json")
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
		t.Fatal("wire differs")
	}
}
