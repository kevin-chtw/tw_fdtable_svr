package match

import (
	"context"
	"errors"
	"sync"

	pitaya "github.com/topfreegames/pitaya/v3/pkg"
)

// PlayerManager 管理玩家
type PlayerManager struct {
	mu      sync.RWMutex
	players map[string]*Player // tableID -> Table
}

// NewPlayerManager 创建玩家管理器
func NewPlayerManager(app pitaya.Pitaya) *PlayerManager {
	return &PlayerManager{
		players: make(map[string]*Player),
	}
}

// GetPlayer 获取玩家实例
func (p *PlayerManager) Load(userID string) (*Player, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	player, ok := p.players[userID]
	if !ok {
		return nil, errors.New("player not found")
	}
	return player, nil
}

func (p *PlayerManager) Store(ctx context.Context, userID string, matchId, tableId int32, score int64) *Player {
	player := NewPlayer(ctx, userID, matchId, tableId, score)
	p.mu.Lock()
	defer p.mu.Unlock()

	p.players[player.ID] = player
	return player
}

func (p *PlayerManager) Delete(userID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.players, userID)
}
