package match

import (
	"context"
	"errors"
	"sync"

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

	player := playerManager.Store(uid, m.conf.MatchID, tableId, m.conf.InitialChips)
	table := NewTable(m, tableId)
	if err := table.create(player, req); err != nil {
		return nil, err
	}

	m.tables.Store(tableId, table)
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

	player := playerManager.Store(uid, m.conf.MatchID, req.Tableid, m.conf.InitialChips)
	t := table.(*Table)
	if err := t.addPlayer(player); err != nil {
		return nil, err
	}
	return &cproto.JoinRoomAck{Tableid: req.Tableid, Desn: t.desn, Properties: t.fdproperty}, nil
}

func (m *Match) HandleGameResult(tableid int32, msg proto.Message) error {
	ack := msg.(*sproto.GameResultAck)

	table, ok := m.tables.Load(tableid)
	if !ok {
		return errors.New("table not found")
	}

	t := table.(*Table)
	err := t.gameResult(ack)
	if err != nil {
		logger.Log.Errorf("Failed to handle game result: %v", err)
	}
	return err
}

func (m *Match) HandleGameOver(tableid int32, msg proto.Message) error {
	ack := msg.(*sproto.GameOverAck)

	table, ok := m.tables.Load(tableid)
	if !ok {
		return errors.New("table not found")
	}

	t := table.(*Table)
	err := t.gameOver(ack)
	if err != nil {
		logger.Log.Errorf("Failed to handle game over: %v", err)
	}
	return err
}

func (m *Match) NewMatchAck(ack proto.Message) (*cproto.MatchAck, error) {
	data, err := anypb.New(ack)
	if err != nil {
		return nil, err
	}

	return &cproto.MatchAck{
		Serverid: m.app.GetServerID(),
		Matchid:  m.conf.MatchID,
		Ack:      data,
	}, nil
}

func (m *Match) netChange(player *Player, online bool) error {
	table, ok := m.tables.Load(player.tableId)
	if !ok {
		return errors.New("table not found")
	}

	t := table.(*Table)
	return t.netChange(player, online)
}
