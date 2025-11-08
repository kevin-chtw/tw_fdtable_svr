package match

import (
	"context"
	"errors"

	"github.com/kevin-chtw/tw_common/matchbase"
	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
	"google.golang.org/protobuf/proto"
)

type Match struct {
	*matchbase.Match
}

func NewMatch(app pitaya.Pitaya, file string) *matchbase.Match {
	m := &Match{}
	m.Match = matchbase.NewMatch(app, file, m)
	return m.Match
}

func (m *Match) Tick() {

}

func (m *Match) HandleCreateRoom(ctx context.Context, msg proto.Message) (proto.Message, error) {
	req := msg.(*cproto.CreateRoomReq)
	player, err := m.ValidatePlayer(
		ctx,
		matchbase.WithCheckPlayerNotInMatch(),
		matchbase.WithAllowCreateNewPlayer(),
	)
	if err != nil {
		return nil, err
	}

	table := NewTable(m.Match)
	t := table.Sub.(*Table)
	if err := t.create(player, req); err != nil {
		return nil, err
	}
	m.AddTable(table)
	m.AddMatchPlayer(player)
	return m.NewStartClientAck(player), nil
}

func (m *Match) HandleCancelRoom(ctx context.Context, msg proto.Message) (proto.Message, error) {
	req := msg.(*cproto.CancelRoomReq)
	player, table, err := m.ValidateRequest(
		ctx,
		matchbase.WithTableID(req.Tableid),
		matchbase.RequirePlayerInTable(),
	)
	if err != nil {
		return nil, err
	}
	t := table.Sub.(*Table)
	if err := t.cancel(player); err != nil {
		return nil, err
	}
	return &cproto.CancelRoomAck{Tableid: req.Tableid}, nil
}

func (m *Match) HandleFDResult(ctx context.Context, msg proto.Message) (proto.Message, error) {
	_, table, err := m.ValidateRequest(ctx, matchbase.WithPlayerTable())
	if err != nil {
		return nil, err
	}
	t := table.Sub.(*Table)
	return t.result, nil
}

func (m *Match) HandleExitMatch(ctx context.Context, msg proto.Message) (proto.Message, error) {
	player, table, err := m.ValidateRequest(ctx, matchbase.WithPlayerTable(), matchbase.RequirePlayerInTable())
	if err != nil {
		return nil, err
	}

	t := table.Sub.(*Table)
	if err := t.removePlayer(player); err != nil {
		return nil, err
	}

	if len(table.Players) == 0 {
		m.DelTable(t.ID)
	}
	m.DelMatchPlayer(player.ID)
	return &cproto.ExitMatchAck{}, nil
}

// 处理房卡加入请求
func (m *Match) HandleJoinRoom(ctx context.Context, msg proto.Message) (proto.Message, error) {
	req := msg.(*cproto.JoinRoomReq)
	player, table, err := m.ValidateRequest(
		ctx,
		matchbase.WithTableID(req.Tableid),
		matchbase.WithCheckPlayerNotInMatch(),
		matchbase.WithAllowCreateNewPlayer())
	if err != nil {
		return nil, err
	}

	if err := table.AddPlayer(player); err != nil {
		return nil, err
	}
	m.AddMatchPlayer(player)
	return m.NewStartClientAck(player), nil
}

func (m *Match) HandleGameResult(msg proto.Message) error {
	req := msg.(*sproto.GameResultReq)

	table := m.GetTable(req.Tableid)
	if table == nil {
		return errors.New("table not found")
	}

	t := table.Sub.(*Table)
	err := t.gameResult(req)
	if err != nil {
		logger.Log.Errorf("Failed to handle game result: %v", err)
	}
	return err
}

func (m *Match) HandleGameOver(msg proto.Message) error {
	req := msg.(*sproto.GameOverReq)

	table := m.GetTable(req.Tableid)
	if table == nil {
		return errors.New("table not found")
	}

	t := table.Sub.(*Table)
	t.gameOver()
	m.DelTable(t.ID)
	return nil
}

func (m *Match) HandleNetState(msg proto.Message) error {
	req := msg.(*sproto.NetStateReq)
	p := m.GetMatchPlayer(req.Uid)
	if p == nil {
		return errors.New("player not found")
	}
	table := m.GetTable(p.TableId)
	if table == nil {
		return errors.New("table not found")
	}

	if p.Online == req.Online {
		return nil
	}
	p.Online = req.Online
	t := table.Sub.(*Table)
	return t.NetChange(p, req.Online)
}
