package match

import (
	"errors"
)

// Player 表示游戏中的玩家
type Player struct {
	ID       string // 玩家唯一ID
	isOnline bool   // 玩家在线状态
	matchId  int32
	tableId  int32
	score    int64 // 玩家分数
}

// NewPlayer 创建新玩家实例
func NewPlayer(id string, matchId, tableId int32, score int64) *Player {
	p := &Player{
		ID:       id,
		isOnline: true, // 默认在线状态
		matchId:  matchId,
		tableId:  tableId,
		score:    score,
	}
	return p
}

func (p *Player) NetChange(online bool) error {
	match := matchManager.Get(p.matchId)
	if match == nil {
		return errors.New("match not found for ID")
	}
	if p.isOnline == online {
		return nil
	}
	p.isOnline = online
	return match.netChange(p, online)
}
