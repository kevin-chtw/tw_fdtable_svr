package match

// Player 表示游戏中的玩家
type Player struct {
	ID     string // 玩家唯一ID
	Score  int32  // 玩家积分
	Status int32  // 玩家状态，例如：在线0、离线1等
	InRoom bool   // 玩家是否在房间内
}

// NewPlayer 创建新玩家实例
func NewPlayer(id string) *Player {
	p := &Player{
		ID:     id,
		Score:  0, // 初始积分为0
		Status: 0,
		InRoom: false,
	}
	return p
}
