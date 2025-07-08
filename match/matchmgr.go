package match

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	pitaya "github.com/topfreegames/pitaya/v3/pkg"
)

// MatchManager 管理玩家
type MatchManager struct {
	mu     sync.RWMutex
	matchs map[string]*Match // tableID -> Table
	app    pitaya.Pitaya
}

// NewMatchManager 创建玩家管理器
func NewMatchManager(app pitaya.Pitaya) *MatchManager {
	matchmgr := &MatchManager{
		matchs: make(map[string]*Match),
		app:    app,
	}
	matchmgr.LoadMatchs()
	return matchmgr
}

func (pm *MatchManager) LoadMatchs() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 获取所有比赛配置文件
	files, err := filepath.Glob(filepath.Join("etc", "islandmatch", "*.yaml"))
	if err != nil {
		return fmt.Errorf("failed to list match config files: %v", err)
	}

	// 加载每个配置文件
	for _, file := range files {
		config, err := LoadConfig(file)
		if err != nil {
			return fmt.Errorf("failed to load match config %s: %v", file, err)
		}

		// 从文件名提取match ID
		base := filepath.Base(file)
		matchID := strings.TrimSuffix(base, filepath.Ext(base))

		// 创建比赛实例
		match := NewMatch(pm.app, matchID, *config)
		pm.matchs[matchID] = match
	}

	return nil
}

func (pm *MatchManager) Get(matchId string) (*Match, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	match, ok := pm.matchs[matchId]
	if !ok {
		return nil, errors.New("match not found")
	}
	return match, nil
}
