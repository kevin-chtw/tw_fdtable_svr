package match

import (
	"context"
	"errors"
	"math/rand"
	"sync"

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
	app      pitaya.Pitaya
	tables   sync.Map
	conf     *MatchConfig
	tableIds *TableIDs
}

func NewMatch(app pitaya.Pitaya, config *MatchConfig) *Match {
	return &Match{
		app:      app,
		tables:   sync.Map{},
		conf:     config,
		tableIds: NewTableIDs(),
	}
}

// 处理房卡创建请求
func (m *Match) HandleCreateRoom(ctx context.Context, msg proto.Message) (proto.Message, error) {
	req := msg.(*cproto.CreateRoomReq)
	uid := m.app.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		return nil, errors.New("no logged in")
	}

	if player, _ := playerManager.Load(uid); player != nil {
		return nil, errors.New("player is in match")
	}

	tableId, err := m.tableIds.Take()
	if err != nil {
		return nil, errors.New("table is full or no table id")
	}

	player := playerManager.Store(ctx, uid, m.conf.MatchID, tableId, m.conf.InitialChips)
	table := NewTable(m, tableId)
	if err := table.create(player, req); err != nil {
		return nil, err
	}

	m.tables.Store(tableId, table)
	m.sendStartClient(player)
	return &cproto.CreateRoomAck{Tableid: tableId, Desn: req.Desn, Properties: req.Properties}, nil
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

	player, err := playerManager.Load(uid)
	if player == nil || err != nil {
		return nil, errors.New("player not found")
	}

	t := table.(*Table)
	if err := t.cancel(player); err != nil {
		return nil, err
	}
	return &cproto.CancelRoomAck{Tableid: req.Tableid}, nil
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
	if player, _ := playerManager.Load(uid); player != nil {
		return nil, errors.New("player is in match")
	}

	player := playerManager.Store(ctx, uid, m.conf.MatchID, req.Tableid, m.conf.InitialChips)
	t := table.(*Table)
	if err := t.addPlayer(player); err != nil {
		return nil, err
	}
	m.sendStartClient(player)
	return &cproto.JoinRoomAck{Tableid: req.Tableid, Desn: t.desn, Properties: t.fdproperty}, nil
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
		return errors.New("table not found")
	}

	t := table.(*Table)
	err := t.gameOver(req)
	if err != nil {
		logger.Log.Errorf("Failed to handle game over: %v", err)
	}
	return err
}

func (m *Match) NewMatchAck(ctx context.Context, msg proto.Message) ([]byte, error) {
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
	p, err := playerManager.Load(req.Uid)
	if err != nil {
		return err
	}
	if p.isOnline == req.Online {
		return nil
	}
	p.isOnline = req.Online
	table, ok := m.tables.Load(p.tableId)
	if !ok {
		return errors.New("table not found")
	}

	m.sendStartClient(p)
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

func (m *Match) sendStartClient(p *Player) {
	startClientAck := &cproto.StartClientAck{
		MatchType: m.app.GetServer().Type,
		GameType:  m.conf.GameType,
		ServerId:  m.app.GetServerID(),
		MatchId:   m.conf.MatchID,
		TableId:   p.tableId,
	}
	data, err := m.NewMatchAck(p.ctx, startClientAck)
	if err != nil {
		logger.Log.Errorf("Failed to send start client ack: %v", err)
		return
	}
	m.app.SendPushToUsers(m.app.GetServer().Type, data, []string{p.ID}, "proxy")
}
