package main

import (
	"encoding/json"
	"flag"
	"github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/stockcatalog"
	"os"
)

func main() {
	path := flag.String("fixture", "examples/fixtures/stock_conversion_routes.json", "historical parser route capture")
	flag.Parse()
	if flag.NArg() != 0 {
		panic("unexpected positional arguments")
	}
	raw, err := os.ReadFile(*path)
	if err != nil {
		panic(err)
	}
	var doc struct {
		Cases []struct {
			Name      string
			Signature string
			Route     struct {
				Succeeded bool
				Legs      []stockcatalog.StockRouteLeg
			}
		}
	}
	if err = json.Unmarshal(raw, &doc); err != nil {
		panic(err)
	}
	catalog := stockcatalog.ScreenedStockCatalog()
	for _, c := range doc.Cases {
		legs := []stockcatalog.StockRouteLeg{}
		for _, i := range catalog.ConversionLegIndices(c.Route.Succeeded, c.Route.Legs) {
			legs = append(legs, c.Route.Legs[i])
		}
		if err = json.NewEncoder(os.Stdout).Encode(map[string]any{"name": c.Name, "signature": c.Signature, "replay": true, "conversion_legs": legs}); err != nil {
			panic(err)
		}
	}
}
