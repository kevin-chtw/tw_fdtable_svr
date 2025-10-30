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
		Creator:     t.creator.ID,
		Desn:        t.desn,
		Fdproperty:  t.fdproperty,
	}
	_, err := t.send2Game(req)
	return err
}

func (t *Table) cancel(p *Player) error {
	if !t.isOnTable(p) {
		return errors.New("player not on table")
	}
	req := &sproto.CancelTableReq{
		Reason: 1,
	}
	t.gameOver()
	_, err := t.send2Game(req)
	return err
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
	if err = ms.Put(player.ID, t.match.conf.MatchID); err != nil {
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

func (t *Table) send2Game(msg proto.Message) (rsp *sproto.GameAck, err error) {
	data, err := anypb.New(msg)
	if err != nil {
		return nil, err
	}

	req := &sproto.GameReq{
		Matchid: t.match.conf.MatchID,
		Tableid: t.ID,
		Req:     data,
	}
	rsp = &sproto.GameAck{}
	err = t.match.app.RPC(context.Background(), t.match.conf.GameType+".remote.message", rsp, req)
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
		t.match.sendStartClient(player)
	}
	_, err := t.send2Game(req)
	return err
}

func (t *Table) gameResult(msg *sproto.GameResultReq) error {
	for _, p := range msg.Players {
		if player, ok := t.Players[p.Playerid]; ok {
			player.score = p.Score
		}
	}
	return nil
}

func (t *Table) gameOver() {
	t.match.tables.Delete(t.ID)
	t.match.tableIds.PutBack(t.ID)
	module, err := t.match.app.GetModule("matchingstorage")
	if err != nil {
		return
	}
	ms := module.(*storage.ETCDMatching)
	for _, p := range t.Players {
		playerManager.Delete(p.ID)
		if err = ms.Remove(p.ID); err != nil {
			logger.Log.Errorf("Failed to remove player from etcd: %v", err)
			continue
		}
	}
}
