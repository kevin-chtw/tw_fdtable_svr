package match

import (
	"context"
	"errors"
	"time"

	"github.com/kevin-chtw/tw_proto/sproto"
	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/modules"
)

const (
	TableStatusWaiting = iota // 0: waiting
	TableStatusPlaying        // 1: playing
	TableStatusEnded          // 2: ended
)

type Table struct {
	app         pitaya.Pitaya
	ID          int32
	MatchID     int32
	Players     []string
	status      int // 0: waiting, 1: playing, 2: ended
	createdAt   time.Time
	creator     string // 创建者ID
	gameCount   int32  // 游戏局数
	playerCount int32  // 玩家数量
}

func NewTable(app pitaya.Pitaya, matchID, id int32) *Table {
	return &Table{
		ID:          id,
		MatchID:     matchID,
		Players:     []string{},
		status:      TableStatusWaiting,
		createdAt:   time.Now(),
		creator:     "",
		app:         app,
		gameCount:   1,
		playerCount: 4,
	}
}

func (t *Table) Init(creator string, gameCount int32) error {
	t.creator = creator
	t.gameCount = gameCount
	err := t.sendAddTable()
	if err != nil {
		logrus.Errorf("Failed to send AddTableReq: %v", err)
		return err
	}
	t.AddPlayer(t.creator)
	return nil
}

func (t *Table) sendAddTable() error {
	req := t.NewMatch2GameReq()
	req.Req = &sproto.Match2GameReq_AddTableReq{
		AddTableReq: &sproto.AddTableReq{
			GameConfig:  "",
			ScoreBase:   1,
			MatchType:   1, // 默认普通匹配
			GameCount:   t.gameCount,
			PlayerCount: t.playerCount,
		},
	}
	rsp, err := t.SendToGame(req)
	if err != nil {
		logrus.Errorf("Failed to send AddTableReq: %v", err)
		return err
	}

	addTableAck := rsp.GetAddTableAck()
	if addTableAck == nil || addTableAck.ErrorCode != 0 {
		return errors.New("add table ack is nil or error code is not 0")
	}
	return nil
}

// AddPlayer 添加玩家到桌子
func (t *Table) AddPlayer(playerID string) error {
	if len(t.Players) >= int(t.playerCount) {
		return errors.New("table is full")
	}
	for _, p := range t.Players {
		if p == playerID {
			return errors.New("player already exists on table")
		}
	}
	module, err := t.app.GetModule("matchingstorage")
	if err != nil {
		return err
	}
	ms := module.(*modules.ETCDBindingStorage)
	if err = ms.PutBinding(playerID); err != nil {
		return err
	}
	t.Players = append(t.Players, playerID)
	if err := t.sendAddPlayer(playerID, int32(len(t.Players)-1)); err != nil {
		logrus.Errorf("Failed to send AddPlayerReq: %v", err)
		return err
	}
	return nil
}

func (t *Table) sendAddPlayer(playerId string, seat int32) error {
	req := t.NewMatch2GameReq()
	req.Req = &sproto.Match2GameReq_AddPlayerReq{
		AddPlayerReq: &sproto.AddPlayerReq{
			Playerid: playerId,
			Seat:     seat,
			Score:    0, // 初始分数为0
		},
	}
	rsp, err := t.SendToGame(req)
	if err != nil {
		logrus.Errorf("Failed to send AddTableReq: %v", err)
		return err
	}

	addPlayerAck := rsp.GetAddPlayerAck()
	if addPlayerAck == nil || addPlayerAck.ErrorCode != 0 {
		return errors.New("add table ack is nil or error code is not 0")
	}
	return nil
}

func (t *Table) SendToGame(msg *sproto.Match2GameReq) (rsp *sproto.Match2GameAck, err error) {
	rsp = &sproto.Match2GameAck{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = t.app.RPC(ctx, "game.match.message", rsp, msg)
	return
}

func (t *Table) NewMatch2GameReq() *sproto.Match2GameReq {
	return &sproto.Match2GameReq{
		Gameid:  0,
		Matchid: t.MatchID,
		Tableid: t.ID,
	}
}
