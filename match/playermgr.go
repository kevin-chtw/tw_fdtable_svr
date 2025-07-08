package match

import (
	"errors"
	"sync"

	pitaya "github.com/topfreegames/pitaya/v3/pkg"
)

// PlayerManager 管理玩家
type PlayerManager struct {
	mu      sync.RWMutex
	players map[string]*Player // tableID -> Table
	app     pitaya.Pitaya
}

// NewPlayerManager 创建玩家管理器
func NewPlayerManager(app pitaya.Pitaya) *PlayerManager {
	return &PlayerManager{
		players: make(map[string]*Player),
		app:     app,
	}
}

// GetPlayer 获取玩家实例
func (pm *PlayerManager) Load(userID string) (*Player, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	player, ok := pm.players[userID]
	if !ok {
		return nil, errors.New("player not found")
	}
	return player, nil
}

func (pm *PlayerManager) LoadOrStore(userID string) *Player {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	player, ok := pm.players[userID]
	if !ok {
		player = NewPlayer(userID)
		pm.players[userID] = player
	}
	return player
}
