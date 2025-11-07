package match

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"github.com/kevin-chtw/tw_common/matchbase"
	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
	"google.golang.org/protobuf/proto"
)

// Table定义已移动到table.go

type Match struct {
	*matchbase.Match
	tables    sync.Map
	conf      *Config
	Playermgr *matchbase.Playermgr
}

func NewMatch(app pitaya.Pitaya, conf *Config) *Match {
	m := &Match{conf: conf}
	m.Match = matchbase.NewMatch(app, conf.Config, m)
	return m
}

// 处理房卡创建请求
func (m *Match) HandleCreateRoom(ctx context.Context, msg proto.Message) (proto.Message, error) {
	req := msg.(*cproto.CreateRoomReq)
	uid := m.App.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		return nil, errors.New("no logged in")
	}

	if player := m.Playermgr.Load(uid); player != nil {
		return nil, errors.New("player is in match")
	}

	player := matchbase.NewPlayer(nil, ctx, uid, m.conf.Matchid, m.conf.InitialChips)
	m.Playermgr.Store(player)
	table := NewTable(m)
	if err := table.create(player, req); err != nil {
		return nil, err
	}

	m.tables.Store(table.ID, table)
	return m.NewStartClientAck(player), nil
}

func (m *Match) HandleCancelRoom(ctx context.Context, msg proto.Message) (proto.Message, error) {
	req := msg.(*cproto.CancelRoomReq)
	uid := m.App.GetSessionFromCtx(ctx).UID()
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
	uid := m.App.GetSessionFromCtx(ctx).UID()
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
	uid := m.App.GetSessionFromCtx(ctx).UID()
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

	if len(t.Players) == 0 {
		m.tables.Delete(t.ID)
		m.PutBackTableId(t.ID)
	}
	return &cproto.ExitMatchAck{}, nil
}

// 处理房卡加入请求
func (m *Match) HandleJoinRoom(ctx context.Context, msg proto.Message) (proto.Message, error) {
	req := msg.(*cproto.JoinRoomReq)
	uid := m.App.GetSessionFromCtx(ctx).UID()
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

	player := matchbase.NewPlayer(nil, ctx, uid, m.conf.Matchid, m.conf.InitialChips)
	m.Playermgr.Store(player)
	t := table.(*Table)
	if err := t.AddPlayer(player); err != nil {
		return nil, err
	}
	return m.NewStartClientAck(player), nil
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

	t := table.(*Table)
	t.gameOver()
	m.tables.Delete(t.ID)
	m.PutBackTableId(t.ID)
	return nil
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
	return t.NetChange(p, req.Online)
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
