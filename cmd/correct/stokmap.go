package correct

import (
	"sync"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
)

var stokMapLock sync.Mutex

type stokMap map[string]map[string]*stok // file -> id -> stok

func (m stokMap) numberOfTokens() int {
	sum := 0
	for _, x := range m {
		sum += len(x)
	}
	return sum
}

type stok struct {
	internal.Stok
	order int
}

func (m stokMap) get(t apoco.T, withGT bool) *stok {
	stokMapLock.Lock()
	defer stokMapLock.Unlock()
	if _, ok := m[t.File]; !ok {
		m[t.File] = make(map[string]*stok)
	}
	if _, ok := m[t.File][t.ID]; !ok {
		s := &stok{Stok: internal.Stok{OCR: t.Tokens[0]}}
		if withGT {
			s.GT = t.Tokens[len(t.Tokens)-1]
		}
		m[t.File][t.ID] = s
	}
	return m[t.File][t.ID]
}
