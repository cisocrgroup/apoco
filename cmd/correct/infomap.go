package correct

import (
	"fmt"
	"sync"

	"example.com/apoco/pkg/apoco"
)

type info struct {
	ocr, gt, sug             string
	conf                     float64
	rank                     int
	skipped, short, lex, cor bool
}

const infoFormat = "skipped=%t short=%t lex=%t cor=%t rank=%d ocr=%s sug=%s gt=%s"

func (i *info) String() string {
	return fmt.Sprintf(infoFormat,
		i.skipped, i.short, i.lex, i.cor, i.rank, e(i.ocr), e(i.sug), e(i.gt))
}

func e(str string) string {
	if len(str) == 0 {
		return "ε"
	}
	return str
}

var infoMapLock sync.Mutex

type infoMap map[string]map[string]*info // file -> id -> info

func (m infoMap) put(t apoco.Token) *info {
	infoMapLock.Lock()
	defer infoMapLock.Unlock()
	if _, ok := m[t.File]; !ok {
		m[t.File] = make(map[string]*info)
	}
	if _, ok := m[t.File][t.ID]; !ok {
		m[t.File][t.ID] = &info{
			ocr: t.Tokens[0],
			gt:  t.Tokens[len(t.Tokens)-1],
		}
	}
	return m[t.File][t.ID]
}