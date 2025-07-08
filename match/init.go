package match

import pitaya "github.com/topfreegames/pitaya/v3/pkg"

var (
	playerManager *PlayerManager
	matchManager  *MatchManager
)

// InitGame 初始化游戏模块
func InitGame(app pitaya.Pitaya) {
	playerManager = NewPlayerManager(app)
	matchManager = NewMatchManager(app)
}

// GetPlayerManager 获取玩家管理器实例
func GetPlayerManager() *PlayerManager {
	return playerManager
}

func GetMatchManager() *MatchManager {
	return matchManager
}
