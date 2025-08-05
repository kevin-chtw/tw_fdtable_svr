package match

// Player 表示游戏中的玩家
type Player struct {
	ID    string // 玩家唯一ID
	Score int32  // 玩家积分
}

// NewPlayer 创建新玩家实例
func NewPlayer(id string) *Player {
	p := &Player{
		ID:    id,
		Score: 0, // 初始积分为0
	}
	return p
}
