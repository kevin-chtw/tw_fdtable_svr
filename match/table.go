package match

import (
	"errors"
	"time"

	"github.com/kevin-chtw/tw_common/matchbase"
	"github.com/kevin-chtw/tw_common/storage"
	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
)

const (
	TableStatusWaiting = iota // 0: waiting
	TableStatusPlaying        // 1: playing
	TableStatusEnded          // 2: ended
)

type Table struct {
	*matchbase.Table
	status     int // 0: waiting, 1: playing, 2: ended
	createdAt  time.Time
	creator    *matchbase.Player // 创建者ID
	fdproperty map[string]int32
	result     *cproto.FDResultAck
}

func NewTable(match *Match) *Table {
	t := &Table{
		Table:      matchbase.NewTable(match.Match),
		status:     TableStatusWaiting,
		createdAt:  time.Now(),
		creator:    nil,
		fdproperty: make(map[string]int32),
	}
	t.result = &cproto.FDResultAck{Tableid: t.ID}
	return t
}

func (t *Table) create(p *matchbase.Player, req *cproto.CreateRoomReq) error {
	t.result.GameCount = req.GameCount
	t.result.OwnerId = p.ID
	t.result.Desn = req.Desn
	t.creator = p
	t.fdproperty = req.Properties
	t.SendAddTableReq(t.result.GameCount, t.fdproperty)
	return t.AddPlayer(t.creator)
}

func (t *Table) cancel(p *matchbase.Player) error {
	if !t.IsOnTable(p) {
		return errors.New("player not on table")
	}
	t.gameOver()
	t.SendCancelTableReq()
	return nil
}

func (t *Table) removePlayer(p *matchbase.Player) error {
	if !t.IsOnTable(p) {
		return errors.New("player not on table")
	}

	if err := t.SendExitTableReq(p); err != nil {
		return err
	}

	if len(t.Players) == 1 {
		return t.cancel(p)
	} else {
		delete(t.Players, p.ID)
		t.Match.Playermgr.Delete(p.ID)
		module, err := t.Match.App.GetModule("matchingstorage")
		if err != nil {
			return err
		}
		ms := module.(*storage.ETCDMatching)
		return ms.Remove(p.ID)
	}
}

func (t *Table) gameResult(msg *sproto.GameResultReq) error {
	t.result.PlayerData = msg.PlayerData
	t.result.Rounds = append(t.result.Rounds, msg.RoundData)
	for p, s := range msg.Scores {
		t.Players[p].Score = s
		t.result.Scores[p] = t.Players[p].Score
	}
	t.sendRoundResult(msg.CurGameCount, msg.RoundData)
	return nil
}

func (t *Table) gameOver() {
	t.sendMatchResult()
	module, err := t.Match.App.GetModule("matchingstorage")
	if err != nil {
		return
	}
	ms := module.(*storage.ETCDMatching)
	for _, p := range t.Players {
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
		t.Match.PushMsg(p, roundResult)
	}
}

func (t *Table) sendMatchResult() {
	for _, p := range t.Players {
		t.Match.PushMsg(p, t.result)
	}
}
