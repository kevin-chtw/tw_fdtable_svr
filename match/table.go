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
	fdproperty map[string]int32
	seats      []int32
	result     *cproto.FDResultAck
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
		seats:      make([]int32, 0),
		result: &cproto.FDResultAck{
			Tableid: id,
		},
	}
}

func (t *Table) getSeat() int32 {
	for i := range t.match.conf.PlayerPerTable {
		if !t.isUsed(int32(i)) {
			return int32(i)
		}
	}
	return -1
}

func (t *Table) isUsed(seat int32) bool {
	for _, p := range t.Players {
		if p.seat == seat {
			return true
		}
	}
	return false
}

func (t *Table) create(p *Player, req *cproto.CreateRoomReq) error {
	t.result.GameCount = req.GameCount
	t.result.OwnerId = p.ID
	t.result.Desn = req.Desn
	t.creator = p
	t.fdproperty = req.Properties
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
		GameCount:   t.result.GameCount,
		PlayerCount: t.match.conf.PlayerPerTable,
		Creator:     t.creator.ID,
		Desn:        t.result.Desn,
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
	_, err := t.send2Game(req)
	t.gameOver()
	return err
}

func (t *Table) removePlayer(p *Player) error {
	if !t.isOnTable(p) {
		return errors.New("player not on table")
	}
	req := &sproto.ExitTableReq{
		Playerid: p.ID,
	}
	_, err := t.send2Game(req)
	if err != nil {
		return err
	}

	if len(t.Players) == 1 {
		return t.cancel(p)
	} else {
		delete(t.Players, p.ID)
		playerManager.Delete(p.ID)
		module, err := t.match.app.GetModule("matchingstorage")
		if err != nil {
			return err
		}
		ms := module.(*storage.ETCDMatching)
		return ms.Remove(p.ID)
	}
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
	player.seat = t.getSeat()
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
	t.result.PlayerData = msg.PlayerData
	t.result.Rounds = append(t.result.Rounds, msg.RoundData)
	for p, s := range msg.Scores {
		t.Players[p].score = s
		t.result.Scores[p] = t.Players[p].score
	}
	t.sendRoundResult(msg.CurGameCount, msg.RoundData)
	return nil
}

func (t *Table) gameOver() {
	t.sendMatchResult()
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

func (t *Table) sendRoundResult(curGameCount int32, roundData string) {
	roundResult := &cproto.FDRoundResultAck{
		CurGameCount: curGameCount,
		Scores:       t.result.Scores,
		PlayerData:   t.result.PlayerData,
		RoundData:    roundData,
	}
	for _, p := range t.Players {
		t.match.sendRoundResult(p, roundResult)
	}
}

func (t *Table) sendMatchResult() {
	for _, p := range t.Players {
		t.match.sendMatchResult(p, t.result)
	}
}
