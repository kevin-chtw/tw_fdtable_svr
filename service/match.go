package service

import (
	"context"
	"strconv"

	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
)

type Match struct {
	app       pitaya.Pitaya
	ID        string
	PlayerIDs []string
}

func NewMatch(app pitaya.Pitaya, id string) *Match {
	logrus.Infof("Creating new match with ID: %s", id)
	app.GroupCreate(context.Background(), id)
	return &Match{
		app: app,
		ID:  id,
	}
}

func (m *Match) AddPlayer(ctx context.Context) bool {
	s := m.app.GetSessionFromCtx(ctx)
	fakeUID := s.ID()
	if err := s.Bind(ctx, strconv.Itoa(int(fakeUID))); err != nil {
		logrus.Infof("Failed to bind session %s: %v", s.UID(), err)
	}

	m.app.GroupAddMember(ctx, m.ID, s.UID())
	m.PlayerIDs = append(m.PlayerIDs, s.UID())

	count, err := m.app.GroupCountMembers(ctx, m.ID)
	if err != nil {
		logrus.Infof("Failed to count members in group %s: %v", m.ID, err)
		return false
	}

	uids, err := m.app.GroupMembers(ctx, m.ID)
	if err != nil {
		logrus.Infof("Failed to get members in group %s: %v", m.ID, err)
		return false
	}
	if err := s.Push("matchmsg", &cproto.StartClientAck{Matchid: m.ID, Players: uids}); err != nil {
		logrus.Infof("Failed to push matchmsg to %s: %v", s.UID(), err)
		return false
	}
	logrus.Infof("Group %s has %d members", m.ID, count)
	if count == 1 {
		m.StartMatch()
	}
	return true
}

func (m *Match) RemovePlayer(playerID string) {
	for i, id := range m.PlayerIDs {
		if id == playerID {
			m.PlayerIDs = append(m.PlayerIDs[:i], m.PlayerIDs[i+1:]...)
			break
		}
	}
}

func (m *Match) StartMatch() {
	rsp := &cproto.CommonResponse{Err: cproto.ErrCode_OK}
	if err := m.app.RPC(context.Background(), "game.game.matchmsg", rsp, &sproto.Match2GameReq{
		Serverid: m.app.GetServerID(),
		StartGameReq: &sproto.StartGameReq{
			Matchid: m.ID,
			Tableid: m.ID,
			Players: m.PlayerIDs,
		}}); err != nil {
		logrus.Errorf("Failed to start game for match %s: %v", m.ID, err)
	}

	startclientAck := &cproto.StartClientAck{
		Matchid:      m.ID,
		Gameserverid: m.app.GetServerID(),
		Tableid:      m.ID,
		Players:      m.PlayerIDs,
	}

	if err := m.app.GroupBroadcast(context.Background(), "proxy", m.ID, "matchmsg", startclientAck); err != nil {
		logrus.Errorf("Failed to broadcast startclientack for match %s: %v", m.ID, err)
		return
	}
	logrus.Infof("Match %s started with players: %v", m.ID, m.PlayerIDs)
}
