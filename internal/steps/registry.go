package steps

import (
	"fmt"

	"github.com/AlexShchuka/mirabilis/internal/pipeline"
)

type entry struct {
	reg   pipeline.Registered
	index int
}

var registry []entry

func Register(meta pipeline.StepMeta, impl pipeline.Step) {
	for _, e := range registry {
		if e.reg.Meta.Name == meta.Name {
			panic(fmt.Sprintf("steps: duplicate registration for %q", meta.Name))
		}
	}
	registry = append(registry, entry{
		reg:   pipeline.Registered{Meta: meta, Impl: impl},
		index: len(registry),
	})
}

func BuildSteps() []pipeline.Registered {
	n := len(registry)
	byName := make(map[string]int, n)
	for i, e := range registry {
		byName[e.reg.Meta.Name] = i
	}

	inDegree := make([]int, n)
	adj := make([][]int, n)
	for i, e := range registry {
		for _, dep := range e.reg.Meta.Deps {
			j, ok := byName[dep]
			if !ok {
				panic(fmt.Sprintf("steps: %q depends on unknown step %q", e.reg.Meta.Name, dep))
			}
			adj[j] = append(adj[j], i)
			inDegree[i]++
		}
	}

	var queue []int
	for i := 0; i < n; i++ {
		if inDegree[i] == 0 {
			queue = insertSorted(queue, i)
		}
	}

	result := make([]pipeline.Registered, 0, n)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		result = append(result, registry[cur].reg)
		for _, next := range adj[cur] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = insertSorted(queue, next)
			}
		}
	}
	return result
}

func insertSorted(s []int, idx int) []int {
	regIdx := registry[idx].index
	i := 0
	for i < len(s) && registry[s[i]].index < regIdx {
		i++
	}
	s = append(s, 0)
	copy(s[i+1:], s[i:])
	s[i] = idx
	return s
}

func snapshot() []entry {
	cp := make([]entry, len(registry))
	copy(cp, registry)
	return cp
}

func reset(saved []entry) {
	registry = saved
}
