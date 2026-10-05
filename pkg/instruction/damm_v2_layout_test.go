package instruction

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func assertDammField(t *testing.T, actual reflect.Value, want any, path string) {
	t.Helper()
	switch w := want.(type) {
	case map[string]any:
		for key, value := range w {
			var name string
			for _, part := range strings.Split(key, "_") {
				name += strings.ToUpper(part[:1]) + part[1:]
			}
			field := actual.FieldByName(name)
			if !field.IsValid() {
				t.Fatal("missing", path, key)
			}
			assertDammField(t, field, value, path+"."+key)
		}
	case []any:
		if actual.Len() != len(w) {
			t.Fatal("length", path)
		}
		for i, value := range w {
			assertDammField(t, actual.Index(i), value, path)
		}
	case string:
		var got string
		if actual.Kind() == reflect.Array {
			b := make([]byte, actual.Len())
			for i := range b {
				b[len(b)-1-i] = byte(actual.Index(i).Uint())
			}
			got = new(big.Int).SetBytes(b).String()
		} else {
			got = strconv.FormatUint(actual.Uint(), 10)
		}
		if got != w {
			t.Fatal(path, got, w)
		}
	case json.Number:
		if strconv.FormatUint(actual.Uint(), 10) != string(w) {
			t.Fatal(path, actual, w)
		}
	default:
		t.Fatal("unexpected fixture", path, w)
	}
}
func TestDammV2PinnedBorshLayout(t *testing.T) {
	d, e := os.ReadFile("testdata/damm_v2_layout_rust_5_0_6.json")
	if e != nil {
		t.Fatal(e)
	}
	var fixture struct {
		Payload  string
		Expected map[string]any
	}
	decoder := json.NewDecoder(bytes.NewReader(d))
	decoder.UseNumber()
	if e = decoder.Decode(&fixture); e != nil {
		t.Fatal(e)
	}
	payload, e := base64.StdEncoding.DecodeString(fixture.Payload)
	if e != nil {
		t.Fatal(e)
	}
	if len(payload) != 1104 {
		t.Fatal(len(payload))
	}
	for _, data := range [][]byte{payload, append(append([]byte{}, payload...), make([]byte, 32)...)} {
		p := DecodeMeteoraPool(data)
		if p == nil {
			t.Fatal("decode")
		}
		assertDammField(t, reflect.ValueOf(*p), fixture.Expected, "pool")
	}
	if DecodeMeteoraPool(payload[:1103]) != nil {
		t.Fatal("accepted truncated payload")
	}
}
