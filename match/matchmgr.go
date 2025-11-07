package match

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/kevin-chtw/tw_proto/sproto"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// MatchManager 管理玩家
type MatchManager struct {
	mu     sync.RWMutex
	matchs map[int32]*Match
	app    pitaya.Pitaya
}

// NewMatchManager 创建玩家管理器
func NewMatchManager(app pitaya.Pitaya) *MatchManager {
	matchmgr := &MatchManager{
		mu:     sync.RWMutex{},
		matchs: make(map[int32]*Match),
		app:    app,
	}
	matchmgr.LoadMatchs()

	// 启动40秒定时上报match人数
	go matchmgr.startReportPlayerCount()

	return matchmgr
}

// startReportPlayerCount 启动定时上报match人数
func (m *MatchManager) startReportPlayerCount() {
	ticker := time.NewTicker(40 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.reportPlayerCount()
	}
}

// reportPlayerCount 上报所有match的玩家数量
func (m *MatchManager) reportPlayerCount() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	req := &sproto.TourneyUpdateReq{}
	for matchID, match := range m.matchs {
		info := &sproto.TourneyInfo{
			Id:            matchID,
			Name:          match.conf.Name,
			GameType:      match.conf.GameType,
			MatchType:     "fdtable",
			Serverid:      m.app.GetServerID(),
			SignCondition: match.conf.SignCondition,
			Online:        match.GetPlayerCount(),
		}
		req.Infos = append(req.Infos, info)
	}
	m.sendTourneyReq(req)
}

func (m *MatchManager) LoadMatchs() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取所有比赛配置文件
	files, err := filepath.Glob(filepath.Join("etc", "fdtable", "*.yaml"))
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
		match := NewMatch(m.app, config)
		m.matchs[config.Matchid] = match
	}

	return nil
}

func (m *MatchManager) sendTourneyReq(msg proto.Message) {
	data, err := anypb.New(msg)
	if err != nil {
		logger.Log.Errorf("failed to create anypb: %v", err)
		return
	}

	req := &sproto.TourneyReq{
		Req: data,
	}
	ack := &sproto.TourneyAck{}
	if err = m.app.RPC(context.Background(), "tourney.remote.message", ack, req); err != nil {
		logger.Log.Errorf("failed to register match to tourney: %v", err)
	}
}

func (m *MatchManager) Get(matchId int32) *Match {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.matchs[matchId]
}
