package match

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"github.com/kevin-chtw/tw_common/matchbase"
	"github.com/kevin-chtw/tw_common/utils"
	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// Table定义已移动到table.go

type Match struct {
	app       pitaya.Pitaya
	tables    sync.Map
	conf      *MatchConfig
	Playermgr *matchbase.Playermgr
	tableIds  *matchbase.TableIDs
}

func NewMatch(app pitaya.Pitaya, config *MatchConfig) *Match {
	return &Match{
		app:       app,
		tables:    sync.Map{},
		Playermgr: matchbase.NewPlayermgr(),
		conf:      config,
		tableIds:  matchbase.NewTableIDs(),
	}
}

// 处理房卡创建请求
func (m *Match) HandleCreateRoom(ctx context.Context, msg proto.Message) (proto.Message, error) {
	req := msg.(*cproto.CreateRoomReq)
	uid := m.app.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		return nil, errors.New("no logged in")
	}

	if player := m.Playermgr.Load(uid); player != nil {
		return nil, errors.New("player is in match")
	}

	tableId, err := m.tableIds.Take()
	if err != nil {
		return nil, errors.New("table is full or no table id")
	}
	player := matchbase.NewPlayer(nil, ctx, uid, m.conf.MatchID, tableId, m.conf.InitialChips)
	m.Playermgr.Store(player)
	table := NewTable(m, tableId)
	if err := table.create(player, req); err != nil {
		return nil, err
	}

	m.tables.Store(tableId, table)
	return m.newStartClientAck(player), nil
}

func (m *Match) HandleCancelRoom(ctx context.Context, msg proto.Message) (proto.Message, error) {
	req := msg.(*cproto.CancelRoomReq)
	uid := m.app.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		return nil, errors.New("no logged in")
	}

	table, ok := m.tables.Load(req.Tableid)
	if !ok {
		return nil, errors.New("table not found")
	}

	player := m.Playermgr.Load(uid)
	if player == nil {
		return nil, errors.New("player not found")
	}

	t := table.(*Table)
	if err := t.cancel(player); err != nil {
		return nil, err
	}
	return &cproto.CancelRoomAck{Tableid: req.Tableid}, nil
}

func (m *Match) HandleFDResult(ctx context.Context, msg proto.Message) (proto.Message, error) {
	uid := m.app.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		return nil, errors.New("no logged in")
	}

	player := m.Playermgr.Load(uid)
	if player == nil {
		return nil, errors.New("player not found")
	}

	table, ok := m.tables.Load(player.TableId)
	if !ok {
		return nil, errors.New("table not found")
	}

	t := table.(*Table)
	return t.result, nil
}

func (m *Match) HandleExitMatch(ctx context.Context, msg proto.Message) (proto.Message, error) {
	uid := m.app.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		return nil, errors.New("no logged in")
	}

	player := m.Playermgr.Load(uid)
	if player == nil {
		return nil, errors.New("player not found")
	}

	table, ok := m.tables.Load(player.TableId)
	if !ok {
		return nil, errors.New("table not found")
	}

	t := table.(*Table)
	if err := t.removePlayer(player); err != nil {
		return nil, err
	}
	return &cproto.ExitMatchAck{}, nil
}

// 处理房卡加入请求
func (m *Match) HandleJoinRoom(ctx context.Context, msg proto.Message) (proto.Message, error) {
	req := msg.(*cproto.JoinRoomReq)
	uid := m.app.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		return nil, errors.New("no logged in")
	}

	table, ok := m.tables.Load(req.Tableid)
	if !ok {
		return nil, errors.New("table not found")
	}
	if player := m.Playermgr.Load(uid); player != nil {
		return nil, errors.New("player is in match")
	}

	player := matchbase.NewPlayer(nil, ctx, uid, m.conf.MatchID, req.Tableid, m.conf.InitialChips)
	m.Playermgr.Store(player)
	t := table.(*Table)
	if err := t.addPlayer(player); err != nil {
		return nil, err
	}
	return m.newStartClientAck(player), nil
}

func (m *Match) HandleGameResult(msg proto.Message) error {
	req := msg.(*sproto.GameResultReq)

	table, ok := m.tables.Load(req.Tableid)
	if !ok {
		return errors.New("table not found")
	}

	t := table.(*Table)
	err := t.gameResult(req)
	if err != nil {
		logger.Log.Errorf("Failed to handle game result: %v", err)
	}
	return err
}

func (m *Match) HandleGameOver(msg proto.Message) error {
	req := msg.(*sproto.GameOverReq)

	table, ok := m.tables.Load(req.Tableid)
	if !ok {
		return fmt.Errorf("table:%d not found", req.Tableid)
	}

	table.(*Table).gameOver()
	return nil
}

func (m *Match) NewMatchAck(ctx context.Context, msg proto.Message) ([]byte, error) {
	logger.Log.Infof("ack %s", utils.JsonMarshal.Format(msg))
	data, err := anypb.New(msg)
	if err != nil {
		return nil, err
	}

	out := &cproto.MatchAck{
		Serverid: m.app.GetServerID(),
		Matchid:  m.conf.MatchID,
		Ack:      data,
	}
	return utils.Marshal(ctx, out)
}

func (m *Match) HandleNetState(msg proto.Message) error {
	req := msg.(*sproto.NetStateReq)
	p := m.Playermgr.Load(req.Uid)
	if p == nil {
		return errors.New("player not found")
	}
	if p.Online == req.Online {
		return nil
	}
	p.Online = req.Online
	table, ok := m.tables.Load(p.TableId)
	if !ok {
		return errors.New("table not found")
	}

	t := table.(*Table)
	return t.netChange(p, req.Online)
}

func (m *Match) GetPlayerCount() int32 {
	count := 0
	m.tables.Range(func(_, _ any) bool {
		count++
		return true
	})
	if count <= 0 {
		return 0
	}
	return (int32(count)-1)*m.conf.PlayerPerTable + rand.Int31n(m.conf.PlayerPerTable)
}

func (m *Match) newStartClientAck(p *matchbase.Player) *cproto.StartClientAck {
	return &cproto.StartClientAck{
		MatchType: m.app.GetServer().Type,
		GameType:  m.conf.GameType,
		ServerId:  m.app.GetServerID(),
		MatchId:   m.conf.MatchID,
		TableId:   p.TableId,
	}
}

func (m *Match) sendStartClient(p *matchbase.Player) {
	startClientAck := m.newStartClientAck(p)
	data, err := m.NewMatchAck(p.Ctx, startClientAck)
	if err != nil {
		logger.Log.Errorf("Failed to send start client ack: %v", err)
		return
	}
	m.app.SendPushToUsers(m.app.GetServer().Type, data, []string{p.ID}, "proxy")
}

func (m *Match) sendMatchResult(p *matchbase.Player, result *cproto.FDResultAck) {
	data, err := m.NewMatchAck(p.Ctx, result)
	if err != nil {
		logger.Log.Errorf("Failed to send start client ack: %v", err)
		return
	}
	m.app.SendPushToUsers(m.app.GetServer().Type, data, []string{p.ID}, "proxy")
}

func (m *Match) sendRoundResult(p *matchbase.Player, result *cproto.FDRoundResultAck) {
	data, err := m.NewMatchAck(p.Ctx, result)
	if err != nil {
		logger.Log.Errorf("Failed to send start client ack: %v", err)
		return
	}
	m.app.SendPushToUsers(m.app.GetServer().Type, data, []string{p.ID}, "proxy")
}
