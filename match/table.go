package match

import (
	"context"
	"errors"
	"time"

	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/modules"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	TableStatusWaiting = iota // 0: waiting
	TableStatusPlaying        // 1: playing
	TableStatusEnded          // 2: ended
)

type Table struct {
	app        pitaya.Pitaya
	ID         int32
	MatchID    int32
	Players    []*Player
	status     int // 0: waiting, 1: playing, 2: ended
	createdAt  time.Time
	creator    *Player // 创建者ID
	gameCount  int32   // 游戏局数
	conf       *MatchConfig
	fdproperty map[string]int32
	desn       string
}

func NewTable(app pitaya.Pitaya, matchID, id int32) *Table {
	return &Table{
		ID:         id,
		MatchID:    matchID,
		Players:    make([]*Player, 0),
		status:     TableStatusWaiting,
		createdAt:  time.Now(),
		creator:    nil,
		fdproperty: make(map[string]int32),
		app:        app,
	}
}

func (t *Table) create(p *Player, req *cproto.CreateRoomReq, config *MatchConfig) error {
	t.creator = p
	t.gameCount = req.GameCount
	t.conf = config
	t.fdproperty = req.Properties
	t.desn = req.Desn
	err := t.sendAddTable()
	if err != nil {
		logrus.Errorf("Failed to send AddTableReq: %v", err)
		return err
	}
	t.AddPlayer(t.creator)
	return nil
}

func (t *Table) cancel(p *Player) error {
	// TODO
	return nil
}

func (t *Table) sendAddTable() error {
	req := &sproto.AddTableReq{
		Property:    t.conf.Property,
		ScoreBase:   int32(t.conf.ScoreBase),
		MatchType:   int32(t.conf.MatchType),
		GameCount:   t.gameCount,
		PlayerCount: int32(t.conf.PlayerPerTable),
		Fdproperty:  t.fdproperty,
	}
	rsp, err := t.send2Game(req)
	if err != nil {
		return err
	}

	ack, err := rsp.Ack.UnmarshalNew()
	if err != nil || ack == nil {
		return err
	}
	return nil
}

// AddPlayer 添加玩家到桌子
func (t *Table) AddPlayer(player *Player) error {
	if len(t.Players) >= int(t.conf.PlayerPerTable) {
		return errors.New("table is full")
	}
	for _, p := range t.Players {
		if p.ID == player.ID {
			return errors.New("player already exists on table")
		}
	}
	module, err := t.app.GetModule("matchingstorage")
	if err != nil {
		return err
	}
	ms := module.(*modules.ETCDBindingStorage)
	if err = ms.PutBinding(player.ID); err != nil {
		return err
	}
	t.Players = append(t.Players, player)
	if err := t.sendAddPlayer(player.ID, int32(len(t.Players)-1)); err != nil {
		logrus.Errorf("Failed to send AddPlayerReq: %v", err)
		return err
	}
	player.InRoom = true
	return nil
}

func (t *Table) sendAddPlayer(playerId string, seat int32) error {
	req := &sproto.AddPlayerReq{
		Playerid: playerId,
		Seat:     seat,
		Score:    int64(t.conf.InitialChips), // 初始分数为0
	}
	rsp, err := t.send2Game(req)
	if err != nil {
		logrus.Errorf("Failed to send AddTableReq: %v", err)
		return err
	}

	ack, err := rsp.Ack.UnmarshalNew()
	if err != nil || ack == nil {
		return err
	}
	return nil
}

func (t *Table) send2Game(msg proto.Message) (rsp *sproto.Match2GameAck, err error) {
	if msg == nil {
		return nil, errors.New("msg is nil")
	}

	data, err := anypb.New(msg)
	if err != nil {
		return nil, err
	}

	req := &sproto.Match2GameReq{
		Gameid:  0,
		Matchid: t.MatchID,
		Tableid: t.ID,
		Req:     data,
	}
	rsp = &sproto.Match2GameAck{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = t.app.RPC(ctx, "game.match.message", rsp, req)
	return
}
