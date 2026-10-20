package stockcatalog

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestCapturedConversionIdentities(t *testing.T) {
	raw, err := os.ReadFile("../../examples/fixtures/stock_conversion_routes.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Cases []struct {
			Name  string
			Route struct {
				Succeeded bool
				Legs      []StockRouteLeg
			}
			ExpectedPools []string `json:"expected_pools"`
		}
	}
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	catalog := ScreenedStockCatalog()
	for _, c := range doc.Cases {
		pools := []string{}
		for _, i := range catalog.ConversionLegIndices(c.Route.Succeeded, c.Route.Legs) {
			pools = append(pools, c.Route.Legs[i].Pool)
		}
		if !reflect.DeepEqual(pools, c.ExpectedPools) {
			t.Fatal(c.Name, pools)
		}
		if len(catalog.ConversionLegIndices(false, c.Route.Legs)) != 0 {
			t.Fatal("failed execution")
		}
		for _, field := range []string{"program", "pool", "protocol", "input", "output"} {
			legs := append([]StockRouteLeg{}, c.Route.Legs...)
			for i := range legs {
				bad := "invalid"
				switch field {
				case "program":
					legs[i].Program = bad
				case "pool":
					legs[i].Pool = bad
				case "protocol":
					legs[i].Protocol = bad
				case "input":
					legs[i].InputMint = &bad
				case "output":
					legs[i].OutputMint = &bad
				}
			}
			if len(catalog.ConversionLegIndices(true, legs)) != 0 {
				t.Fatal("accepted wrong", field)
			}
		}
	}
}
