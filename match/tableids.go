package match

import (
	"errors"
	"math/rand"
	"sync"
)

const (
	minID = 10000
	maxID = 99999
)

var (
	ErrExhausted = errors.New("all ids in range are used")
)

// TableIDs 负责生成不重复的 5 位数字
type TableIDs struct {
	mu      sync.Mutex
	used    map[int32]struct{}
	shuffle []int32
	index   int
}

// NewTableIDs 创建一个新的生成器
func NewTableIDs() *TableIDs {
	n := maxID - minID + 1
	shuffle := make([]int32, n)
	for i := range shuffle {
		shuffle[i] = minID + int32(i)
	}
	// Fisher–Yates shuffle
	rand.Shuffle(n, func(i, j int) {
		shuffle[i], shuffle[j] = shuffle[j], shuffle[i]
	})

	return &TableIDs{
		used:    make(map[int32]struct{}, n),
		shuffle: shuffle,
		index:   0,
	}
}

// Take 返回一个未被使用过的数字；区间耗尽返回 ErrExhausted
func (g *TableIDs) Take() (int32, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for g.index < len(g.shuffle) {
		id := g.shuffle[g.index]
		g.index++
		if _, ok := g.used[id]; !ok {
			g.used[id] = struct{}{}
			return id, nil
		}
	}
	return 0, ErrExhausted
}

// PutBack 把某个数字归还到可用池（可选接口）
func (g *TableIDs) PutBack(id int32) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.used, id)
}
