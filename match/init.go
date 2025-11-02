package match

import pitaya "github.com/topfreegames/pitaya/v3/pkg"

var (
	matchManager *MatchManager
)

// InitGame 初始化游戏模块
func InitGame(app pitaya.Pitaya) {
	matchManager = NewMatchManager(app)
}

func GetMatchManager() *MatchManager {
	return matchManager
}
