package match

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/kevin-chtw/tw_common/matchbase"
	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
)

type Table struct {
	*matchbase.Table
	creator    *matchbase.Player // 创建者ID
	fdproperty map[string]int32
	result     *cproto.FDResultAck
}

func NewTable(match *matchbase.Match) *matchbase.Table {
	t := &Table{
		creator:    nil,
		fdproperty: make(map[string]int32),
	}
	t.Table = matchbase.NewTable(match, t)
	t.result = &cproto.FDResultAck{Tableid: t.ID}

	return t.Table
}

func (t *Table) create(p *matchbase.Player, req *cproto.CreateRoomReq) error {
	conf := req.GetMatchConfig()
	count, ok := conf["player_count"]
	if ok {
		allowCnt := t.Match.Viper.GetIntSlice("allow_player_count")
		if !slices.Contains(allowCnt, int(count)) {
			return fmt.Errorf("allow player count must be in %v", allowCnt)
		} else {
			t.PlayerCount = count
		}
	}

	t.result.GameCount = req.GameCount
	t.result.OwnerId = p.ID
	t.result.Desn = req.Desn
	t.creator = p
	t.fdproperty = req.Properties

	t.SendAddTableReq(t.result.GameCount, t.creator.ID, t.fdproperty)
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

	delete(t.Players, p.ID)
	if len(t.Players) <= 0 {
		return t.cancel(p)
	}
	return nil
}

func (t *Table) gameResult(msg *sproto.GameResultReq) error {
	t.result.PlayerData = msg.PlayerData
	t.result.Rounds = append(t.result.Rounds, msg.RoundData)
	for p, s := range msg.Scores {
		t.Players[p].Score = s
		t.result.Scores[p] = t.Players[p].Score

	}
	maps.Copy(t.result.PlayerData, msg.PlayerData)
	t.sendRoundResult(msg.CurGameCount, msg.RoundData)
	return nil
}

func (t *Table) gameOver() {
	t.sendMatchResult()
	for _, p := range t.Players {
		t.Match.DelMatchPlayer(p.ID)
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
