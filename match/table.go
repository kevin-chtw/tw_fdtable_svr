package match

import (
	"context"
	"errors"
	"time"

	"github.com/kevin-chtw/tw_common/storage"
	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
	"github.com/sirupsen/logrus"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	TableStatusWaiting = iota // 0: waiting
	TableStatusPlaying        // 1: playing
	TableStatusEnded          // 2: ended
)

type Table struct {
	match      *Match
	ID         int32
	Players    map[string]*Player
	status     int // 0: waiting, 1: playing, 2: ended
	createdAt  time.Time
	creator    *Player // 创建者ID
	gameCount  int32   // 游戏局数
	fdproperty map[string]int32
	desn       string
}

func NewTable(match *Match, id int32) *Table {
	return &Table{
		match:      match,
		ID:         id,
		Players:    make(map[string]*Player),
		status:     TableStatusWaiting,
		createdAt:  time.Now(),
		creator:    nil,
		fdproperty: make(map[string]int32),
	}
}

func (t *Table) create(p *Player, req *cproto.CreateRoomReq) error {
	t.creator = p
	t.gameCount = req.GameCount
	t.fdproperty = req.Properties
	t.desn = req.Desn
	err := t.sendCreateTable()
	if err != nil {
		logrus.Errorf("Failed to send AddTableReq: %v", err)
		return err
	}
	return t.addPlayer(t.creator)
}

func (t *Table) sendCreateTable() error {
	req := &sproto.AddTableReq{
		Property:    t.match.conf.Property,
		ScoreBase:   t.match.conf.ScoreBase,
		MatchType:   t.match.app.GetServer().Type,
		GameCount:   t.gameCount,
		PlayerCount: t.match.conf.PlayerPerTable,
		Fdproperty:  t.fdproperty,
	}
	_, err := t.send2Game(req)
	return err
}

func (t *Table) cancel(p *Player) error {
	if !t.isOnTable(p) {
		return errors.New("player not on table")
	}

	return t.gameOver(1)
}

// addPlayer 添加玩家到桌子
func (t *Table) addPlayer(player *Player) error {
	if len(t.Players) >= int(t.match.conf.PlayerPerTable) {
		return errors.New("table is full")
	}

	if t.isOnTable(player) {
		return errors.New("player already exists on table")
	}

	module, err := t.match.app.GetModule("matchingstorage")
	if err != nil {
		return err
	}
	ms := module.(*storage.ETCDMatching)
	if err = ms.Put(player.ID); err != nil {
		return err
	}
	t.Players[player.ID] = player
	if err := t.sendAddPlayer(player.ID, int32(len(t.Players)-1)); err != nil {
		logger.Log.Errorf("Failed to send AddPlayerReq: %v", err)
		return err
	}
	return nil
}

func (t *Table) sendAddPlayer(playerId string, seat int32) error {
	req := &sproto.AddPlayerReq{
		Playerid: playerId,
		Seat:     seat,
		Score:    int64(t.match.conf.InitialChips),
	}
	_, err := t.send2Game(req)
	return err
}

func (t *Table) isOnTable(player *Player) bool {
	for _, p := range t.Players {
		if p.ID == player.ID {
			return true
		}
	}
	return false
}

func (t *Table) send2Game(msg proto.Message) (rsp *sproto.Match2GameAck, err error) {
	data, err := anypb.New(msg)
	if err != nil {
		return nil, err
	}

	req := &sproto.Match2GameReq{
		Gameid:  t.match.conf.GameID,
		Matchid: t.match.conf.MatchID,
		Tableid: t.ID,
		Req:     data,
	}
	rsp = &sproto.Match2GameAck{}
	err = t.match.app.RPC(context.Background(), t.match.conf.Route, rsp, req)
	return
}

func (t *Table) netChange(player *Player, online bool) error {
	if !t.isOnTable(player) {
		return errors.New("player not on table")
	}
	req := &sproto.NetStateReq{
		Uid:    player.ID,
		Online: online,
	}
	if online {
		rsp := &cproto.JoinRoomAck{Tableid: t.ID, Desn: t.desn, Properties: t.fdproperty}
		msg, err := t.match.NewMatchAck(rsp)
		if err != nil {
			return err
		}
		t.send2User(msg, []string{player.ID})
	}
	_, err := t.send2Game(req)
	return err
}

func (t *Table) send2User(msg *cproto.MatchAck, players []string) {
	if m, err := t.match.app.SendPushToUsers(t.match.app.GetServer().Type, msg, players, "proxy"); err != nil {
		logger.Log.Errorf("send game message to player %v failed: %v", players, err)
	} else {
		logger.Log.Infof("send game message to player %v: %v", players, m)
	}
}

func (t *Table) gameResult(msg *sproto.GameResultAck) error {
	for _, p := range msg.Players {
		if player, ok := t.Players[p.Playerid]; ok {
			player.score = p.Score
		}
	}
	if msg.IsOver || msg.CurGameCount >= t.gameCount {
		return t.gameOver(0)
	}

	return nil
}

func (t *Table) gameOver(reason int32) error {
	req := &sproto.CancelTableReq{
		Reason: reason,
	}
	if _, err := t.send2Game(req); err != nil {
		return err
	}
	module, err := t.match.app.GetModule("matchingstorage")
	if err != nil {
		return err
	}
	ms := module.(*storage.ETCDMatching)
	for _, p := range t.Players {
		playerManager.Delete(p.ID)
		if err = ms.Remove(p.ID); err != nil {
			logger.Log.Errorf("Failed to remove player from etcd: %v", err)
			continue
		}
	}

	t.match.tables.Delete(t.ID)
	t.match.tableIds.PutBack(t.ID)
	return nil
}
