package match

import (
	"path/filepath"

	"github.com/kevin-chtw/tw_common/matchbase"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
)

// MatchManager 管理玩家
type MatchManager struct {
	*matchbase.Matchmgr
}

// NewMatchManager 创建玩家管理器
func NewMatchManager(app pitaya.Pitaya) *MatchManager {
	matchmgr := &MatchManager{
		Matchmgr: matchbase.NewMatchmgr(app),
	}
	matchmgr.LoadMatchs()
	return matchmgr
}

func (m *MatchManager) LoadMatchs() error {
	// 获取所有比赛配置文件
	files, err := filepath.Glob(filepath.Join("etc", m.App.GetServer().Type, "*.yaml"))
	if err != nil {
		return err
	}

	// 加载每个配置文件
	for _, file := range files {
		config, err := LoadConfig(file)
		if err != nil {
			logger.Log.Error(err.Error())
			continue
		}

		logger.Log.Infof("加载比赛配置: %s", file)
		match := NewMatch(m.App, config)
		m.Matchmgr.Add(match.Match)
	}

	return nil
}

func (m *MatchManager) Get(matchId int32) *Match {
	match := m.Matchmgr.Get(matchId)
	if match == nil {
		return nil
	}
	return match.Sub.(*Match)
}
