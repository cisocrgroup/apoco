package correct

import (
	"sync"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
)

var infoMapLock sync.Mutex

type infoMap map[string]map[string]*internal.Stok // file -> id -> stok

func (m infoMap) numberOfTokens() int {
	sum := 0
	for _, x := range m {
		sum += len(x)
	}
	return sum
}

func (m infoMap) get(t apoco.T) *internal.Stok {
	infoMapLock.Lock()
	defer infoMapLock.Unlock()
	if _, ok := m[t.File]; !ok {
		m[t.File] = make(map[string]*internal.Stok)
	}
	if _, ok := m[t.File][t.ID]; !ok {
		m[t.File][t.ID] = &internal.Stok{
			OCR: t.Tokens[0],
			GT:  t.Tokens[len(t.Tokens)-1],
		}
	}
	return m[t.File][t.ID]
}
