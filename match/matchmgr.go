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

// 获取玩家创建的房卡匹配
func (pm *MatchManager) GetPlayerRoomCardMatches(creator string) []*Match {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var matches []*Match
	for _, match := range pm.matchs {
		if match.MatchType == 1 && match.Creator == creator {
			matches = append(matches, match)
		}
	}
	return matches
}

// 通过邀请码获取房卡匹配
func (pm *MatchManager) GetMatchByInviteCode(inviteCode string) (*Match, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, match := range pm.matchs {
		if match.MatchType == 1 && match.InviteCode == inviteCode {
			return match, nil
		}
	}
	return nil, errors.New("match not found")
}

// AddMatch 添加一个新的匹配到管理器
func (pm *MatchManager) AddMatch(match *Match) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.matchs[match.ID]; exists {
		return fmt.Errorf("match with ID %s already exists", match.ID)
	}

	pm.matchs[match.ID] = match
	return nil
}
