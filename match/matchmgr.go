package match

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	pitaya "github.com/topfreegames/pitaya/v3/pkg"
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
		matchs: make(map[int32]*Match),
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
		matchIDInt, err := strconv.ParseInt(matchID, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid match ID format %s: %v", matchID, err)
		}
		match := NewMatch(pm.app, int32(matchIDInt), *config)
		pm.matchs[int32(matchIDInt)] = match
	}

	return nil
}

func (pm *MatchManager) Get(matchId int32) *Match {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.matchs[matchId]
}

// AddMatch 添加一个新的匹配到管理器
func (pm *MatchManager) AddMatch(match *Match) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.matchs[match.ID]; exists {
		return fmt.Errorf("match with ID %d already exists", match.ID)
	}

	pm.matchs[match.ID] = match
	return nil
}
