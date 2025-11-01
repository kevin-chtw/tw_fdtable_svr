package match

import (
	"context"
)

// Player 表示游戏中的玩家
type Player struct {
	ctx      context.Context
	ID       string // 玩家唯一ID
	isOnline bool   // 玩家在线状态
	matchId  int32
	tableId  int32
	score    int64 // 玩家分数
	seat     int32
}

// NewPlayer 创建新玩家实例
func NewPlayer(ctx context.Context, id string, matchId, tableId int32, score int64) *Player {
	p := &Player{
		ctx:      ctx,
		ID:       id,
		isOnline: true, // 默认在线状态
		matchId:  matchId,
		tableId:  tableId,
		score:    score,
		seat:     -1,
	}
	return p
}
