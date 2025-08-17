package match

import (
	"path/filepath"
	"sync"

	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
)

// MatchManager 管理玩家
type MatchManager struct {
	mu     sync.RWMutex
	matchs map[int32]*Match
	app    pitaya.Pitaya
}

// NewMatchManager 创建玩家管理器
func NewMatchManager(app pitaya.Pitaya) *MatchManager {
	matchmgr := &MatchManager{
		mu:     sync.RWMutex{},
		matchs: make(map[int32]*Match),
		app:    app,
	}
	matchmgr.LoadMatchs()
	return matchmgr
}

func (m *MatchManager) LoadMatchs() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取所有比赛配置文件
	files, err := filepath.Glob(filepath.Join("etc", "islandmatch", "*.yaml"))
	if err != nil {
		return err
	}

	// 加载每个配置文件
	for _, file := range files {
		config, err := LoadConfig(file)
		if err != nil {
			logger.Log.Error(err.Error())
			continue
		}

		logger.Log.Infof("加载比赛配置: %s", file)
		match := NewMatch(m.app, config)
		m.matchs[config.MatchID] = match
	}

	return nil
}

func (m *MatchManager) Get(matchId int32) *Match {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.matchs[matchId]
}
