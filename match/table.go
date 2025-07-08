package match

import (
	"context"
	"errors"
	"time"

	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
)

type Table struct {
	ID        string
	Players   []string
	Status    int // 0: waiting, 1: playing, 2: ended
	CreatedAt time.Time
	MatchID   string
	App       pitaya.Pitaya
}

// HandleMatchResult 处理从game服返回的Match2GameAck
func (t *Table) HandleMatchResult(ctx context.Context, ack *sproto.Match2GameAck) error {
	return t.HandleGameResult(ack)
}

func (t *Table) ToProto() *cproto.TableInfo {
	return &cproto.TableInfo{
		Tableid:  t.ID,
		Players:  t.Players,
		Status:   int32(t.Status),
		CreateAt: t.CreatedAt.Unix(),
	}
}

func NewTable(app pitaya.Pitaya, matchID, id string, players []string) *Table {
	app.GroupCreate(context.Background(), id)
	return &Table{
		ID:        id,
		MatchID:   matchID,
		Players:   players,
		Status:    0, // waiting
		CreatedAt: time.Now(),
		App:       app,
	}
}

// AddPlayer 添加玩家到桌子
func (t *Table) AddPlayer(playerID string) bool {
	if t.Status != 0 {
		return false // 桌子不在等待状态
	}

	for _, p := range t.Players {
		if p == playerID {
			return false // 玩家已在桌
		}
	}

	t.Players = append(t.Players, playerID)
	return true
}

// StartGame 通知游戏服务开始游戏
func (t *Table) StartGame(config MatchConfig) error {
	t.Status = 1 // playing

	gameReq := &sproto.Match2GameReq{
		StartGameReq: &sproto.StartGameReq{
			MatchServerId: t.App.GetServerID(),
			Matchid:       t.MatchID,
			Tableid:       t.ID,
			Players:       t.Players,
			GameConfig:    config.GameConfig.Rules,
			InitialChips:  int32(config.InitialChips),
			ScoreBase:     int32(config.ScoreBase),
		},
	}

	rsp := &cproto.CommonResponse{Err: cproto.ErrCode_OK}
	if err := t.App.RPC(context.Background(), "game.game.matchmsg", rsp, gameReq); err != nil {
		t.Status = 0 // 回滚状态
		return err
	}

	// 通知玩家进入桌子
	startAck := &cproto.StartClientAck{
		Matchid: t.MatchID,
		Tableid: t.ID,
		Players: t.Players,
	}

	if err := t.App.GroupBroadcast(context.Background(), "proxy", t.ID, "matchmsg", startAck); err != nil {
		return err
	}

	logrus.Infof("Table %s started with players: %v", t.ID, t.Players)
	return nil
}

// HandleGameResult 处理游戏结果
func (t *Table) HandleGameResult(ack *sproto.Match2GameAck) error {
	if ack.GameResultAck == nil {
		return errors.New("invalid game result ack")
	}

	t.Status = 2 // ended
	result := ack.GameResultAck

	// 广播游戏结果
	endAck := &cproto.GameEndAck{
		Matchid:   t.MatchID,
		Tableid:   t.ID,
		Players:   t.Players,
		EndReason: result.EndReason,
		Winner:    getWinnerFromScores(result.Scores), // 根据分数计算获胜者
	}

	if err := t.App.GroupBroadcast(context.Background(), "proxy", t.MatchID, "matchmsg", endAck); err != nil {
		return err
	}

	// 归还玩家到匹配池
	for _, playerID := range t.Players {
		t.App.GroupRemoveMember(context.Background(), t.MatchID, playerID)
	}

	logrus.Infof("Table %s ended with result: %+v", t.ID, result)
	return nil
}

// IsFull 检查桌子是否已满
func (t *Table) IsFull(maxPlayers int) bool {
	return len(t.Players) >= maxPlayers
}

// getWinnerFromScores 根据分数数组返回获胜者索引
func getWinnerFromScores(scores []int32) int32 {
	if len(scores) == 0 {
		return -1
	}
	maxIndex := 0
	for i := 1; i < len(scores); i++ {
		if scores[i] > scores[maxIndex] {
			maxIndex = i
		}
	}
	return int32(maxIndex)
}
