package calc

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strconv"
	"testing"
)

func TestDlmmSparsePreviousReference(t *testing.T) {
	data, err := os.ReadFile("testdata/dlmm_review_reference_20261005.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		dlmmFixture
		Name   string
		Strict bool
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			pool, original, amount, ts := dlmmArgs(t, c.dlmmFixture)
			ascending := append([]DlmmBin(nil), original...)
			sort.Slice(ascending, func(i, j int) bool { return ascending[i].BinID < ascending[j].BinID })
			reversed := append([]DlmmBin(nil), original...)
			for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
				reversed[i], reversed[j] = reversed[j], reversed[i]
			}
			for _, bins := range [][]DlmmBin{original, ascending, reversed} {
				before, _ := json.Marshal(bins)
				q, err := DlmmSwapExactIn(pool, bins, c.Arrays, amount, ts, c.Down, c.Orders, c.Strict, c.Exhaustive)
				var missing *InsufficientDlmmArrays
				partial := errors.As(err, &missing)
				if err != nil && !partial {
					t.Fatal(err)
				}
				want := c.Expected
				if strconv.FormatUint(q.AmountOut, 10) != want.Out || strconv.FormatUint(q.RemainingIn, 10) != want.Remaining || q.BinsCrossed != want.Crossed || q.Complete != want.Complete || partial != want.Error {
					t.Fatal(q, want, err)
				}
				if (q.MissingBinID == nil) != (want.Missing == nil) || q.MissingBinID != nil && *q.MissingBinID != *want.Missing {
					t.Fatal("missing bin changed")
				}
				after, _ := json.Marshal(bins)
				if string(before) != string(after) {
					t.Fatal("mutated bins")
				}
			}
		})
	}
}
