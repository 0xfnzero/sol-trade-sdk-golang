package subscription

import (
	"errors"
	"github.com/gagliardetto/solana-go"
	"sort"
)

// CandidateRoutes returns increasing-hop paths. Actual pool state is validated by PrepareRoute.
func VisitCandidateRoutes(candidates []PoolTradeHint, input, output solana.PublicKey, maximumHops int, visit func([]PoolTradeHint) (bool, error)) error {
	if visit == nil {
		return errors.New("missing candidate visitor")
	}
	if maximumHops < 1 || maximumHops > 5 || input == output || input.IsZero() || output.IsZero() {
		return errors.New("invalid candidate route endpoints or hop limit")
	}
	pools := map[solana.PublicKey]PoolTradeHint{}
	for _, h := range candidates {
		if err := h.validate(); err != nil {
			return err
		}
		if old, ok := pools[h.Pool]; ok && !((old.InputMint == h.InputMint && old.OutputMint == h.OutputMint) || (old.InputMint == h.OutputMint && old.OutputMint == h.InputMint)) {
			return errors.New("conflicting candidate pool mints")
		}
		pools[h.Pool] = h
		if len(pools) > 64 {
			return errors.New("candidate search budget: maximum 64 pools")
		}
	}
	type edge struct {
		hint         PoolTradeHint
		pool, output string
	}
	edges := []edge{}
	for _, h := range pools {
		name := h.Pool.String()
		edges = append(edges, edge{h, name, h.OutputMint.String()}, edge{PoolTradeHint{h.Pool, h.OutputMint, h.InputMint}, name, h.InputMint.String()})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].pool == edges[j].pool {
			return edges[i].output < edges[j].output
		}
		return edges[i].pool < edges[j].pool
	})
	adjacent := map[solana.PublicKey][]PoolTradeHint{}
	for _, e := range edges {
		adjacent[e.hint.InputMint] = append(adjacent[e.hint.InputMint], e.hint)
	}
	type path struct {
		hints []PoolTradeHint
		mint  solana.PublicKey
		seen  map[solana.PublicKey]bool
	}
	frontier := []path{{nil, input, map[solana.PublicKey]bool{input: true}}}
	routes := 0
	for depth := 0; depth < maximumHops && len(frontier) > 0; depth++ {
		next := []path{}
		for _, p := range frontier {
			for _, h := range adjacent[p.mint] {
				if p.seen[h.OutputMint] {
					continue
				}
				reused := false
				for _, x := range p.hints {
					if x.Pool == h.Pool {
						reused = true
					}
				}
				if reused {
					continue
				}
				hints := append(append([]PoolTradeHint{}, p.hints...), h)
				if h.OutputMint == output {
					routes++
					if routes > 64 {
						return errors.New("candidate search budget: maximum 64 paths")
					}
					stop, err := visit(hints)
					if err != nil {
						return err
					}
					if stop {
						return nil
					}
				} else if depth+1 < maximumHops {
					if len(next) >= 4096 {
						return errors.New("candidate search budget exceeded")
					}
					seen := map[solana.PublicKey]bool{}
					for k := range p.seen {
						seen[k] = true
					}
					seen[h.OutputMint] = true
					next = append(next, path{hints, h.OutputMint, seen})
				}
			}
		}
		frontier = next
	}
	if routes == 0 {
		return errors.New("no connected candidate route")
	}
	return nil
}

// CandidateRoutes collects bounded shortest paths. Preparation uses VisitCandidateRoutes to stop at the first valid route.
func CandidateRoutes(candidates []PoolTradeHint, input, output solana.PublicKey, maximumHops int) ([][]PoolTradeHint, error) {
	routes := [][]PoolTradeHint{}
	err := VisitCandidateRoutes(candidates, input, output, maximumHops, func(hints []PoolTradeHint) (bool, error) { routes = append(routes, hints); return false, nil })
	if err != nil {
		return nil, err
	}
	return routes, nil
}
