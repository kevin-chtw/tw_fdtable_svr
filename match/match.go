package match

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
)

// Table定义已移动到table.go

type Match struct {
	app          pitaya.Pitaya
	ID           string
	PlayerIDs    []string
	Tables       map[string]*Table
	Config       MatchConfig
	Status       int // 0: waiting, 1: matching, 2: playing, 3: ended
	CreatedAt    time.Time
	ReadyPlayers map[string]bool
	mu           sync.RWMutex // 新增读写锁
}

func NewMatch(app pitaya.Pitaya, id string, config MatchConfig) *Match {
	logrus.Infof("Creating new mahjong match with ID: %s", id)
	logrus.Infof("Match config: %+v", config)

	app.GroupCreate(context.Background(), id)

	match := &Match{
		app:          app,
		ID:           id,
		Config:       config,
		Status:       0, // waiting
		CreatedAt:    time.Now(),
		ReadyPlayers: make(map[string]bool),
		Tables:       make(map[string]*Table),
	}

	return match
}

func (m *Match) HandleMatchReq(ctx context.Context, req *cproto.MatchReq) *cproto.MatchAck {
	ack := &cproto.MatchAck{
		Serverid: m.app.GetServerID(),
	}

	// Handle signup request
	if signupReq := req.GetSignupReq(); signupReq != nil {
		signupAck := m.HandleSignup(ctx, signupReq)
		ack.Ack = &cproto.MatchAck_SignupAck{SignupAck: signupAck}
	}

	return ack
}

func (m *Match) HandleSignup(ctx context.Context, req *cproto.SignupReq) *cproto.SignupAck {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Status != 0 {
		return &cproto.SignupAck{
			Matchid:   req.Matchid,
			ErrorCode: 1, // match not in waiting status
		}
	}

	if len(m.PlayerIDs) >= m.Config.MaxPlayers {
		return &cproto.SignupAck{
			Matchid:   req.Matchid,
			ErrorCode: 3, // match full
		}
	}

	// Add player to match
	uid := m.app.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		logrus.Warnf("Player ID is empty for signup request: %v", req)
		return &cproto.SignupAck{
			Matchid:   req.Matchid,
			ErrorCode: 4, // invalid player ID
		}
	}
	m.PlayerIDs = append(m.PlayerIDs, uid)
	m.ReadyPlayers[uid] = false

	logrus.Infof("Player %s joined match %s, current players: %d/%d",
		uid, m.ID, len(m.PlayerIDs), m.Config.MaxPlayers)

	// Check if match is full
	if len(m.PlayerIDs) == m.Config.MaxPlayers {
		go m.StartMatch()
	}

	return &cproto.SignupAck{
		Matchid:   req.Matchid,
		ErrorCode: 0, // success
	}
}

func (m *Match) HandleCancelReq(ctx context.Context, req *cproto.MatchCancelReq) *cproto.MatchCancelAck {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, id := range m.PlayerIDs {
		if id == req.Playerid {
			m.PlayerIDs = append(m.PlayerIDs[:i], m.PlayerIDs[i+1:]...)
			delete(m.ReadyPlayers, req.Playerid)
			return &cproto.MatchCancelAck{
				Matchid:   req.Matchid,
				ErrorCode: 0, // success
			}
		}
	}

	return &cproto.MatchCancelAck{
		Matchid:   req.Matchid,
		ErrorCode: 2, // player not in match
	}
}

func (m *Match) HandleStatusReq(ctx context.Context, req *cproto.MatchStatusReq) *cproto.MatchStatusAck {
	// Convert tables to proto format
	var tables []*cproto.TableInfo
	for _, table := range m.Tables {
		tables = append(tables, table.ToProto())
	}

	return &cproto.MatchStatusAck{
		Matchid: req.Matchid,
		Status:  int32(m.Status),
		Players: m.PlayerIDs,
		Timeout: int32(m.Config.Timeout.Seconds() - time.Since(m.CreatedAt).Seconds()),
		Tables:  tables,
	}
}

func (m *Match) HandlePlayerReady(ctx context.Context, notify *cproto.PlayerReadyNotify) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.ReadyPlayers[notify.Playerid]; exists {
		m.ReadyPlayers[notify.Playerid] = notify.IsReady
	}

	// Check if all players are ready
	allReady := true
	for _, ready := range m.ReadyPlayers {
		if !ready {
			allReady = false
			break
		}
	}

	if allReady && len(m.PlayerIDs) >= m.Config.MinPlayers {
		m.StartMatch()
	}
}

func (m *Match) AddPlayer(ctx context.Context) bool {
	s := m.app.GetSessionFromCtx(ctx)
	fakeUID := s.ID()
	if err := s.Bind(ctx, strconv.Itoa(int(fakeUID))); err != nil {
		logrus.Warnf("Failed to bind session %s: %v", s.UID(), err)
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.app.GroupAddMember(ctx, m.ID, s.UID()); err != nil {
		logrus.Warnf("Failed to add member %s to group %s: %v", s.UID(), m.ID, err)
		return false
	}
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
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, id := range m.PlayerIDs {
		if id == playerID {
			m.PlayerIDs = append(m.PlayerIDs[:i], m.PlayerIDs[i+1:]...)
			delete(m.ReadyPlayers, playerID)
			break
		}
	}
}

func (m *Match) StartMatch() {
	m.mu.Lock()
	m.Status = 2 // playing
	m.mu.Unlock()

	logrus.Infof("Starting mahjong match %s with %d players", m.ID, len(m.PlayerIDs))

	// Distribute players to tables with skill-based matching
	tableCount := len(m.PlayerIDs) / m.Config.MaxPlayers
	if len(m.PlayerIDs)%m.Config.MaxPlayers != 0 {
		tableCount++
	}

	// Sort players by skill level (placeholder - replace with actual skill data)
	sortedPlayers := make([]string, len(m.PlayerIDs))
	copy(sortedPlayers, m.PlayerIDs)
	sort.Slice(sortedPlayers, func(i, j int) bool {
		return sortedPlayers[i] < sortedPlayers[j] // Replace with actual skill comparison
	})

	for i := 0; i < tableCount; i++ {
		// Distribute players evenly across tables
		var tablePlayers []string
		for j := i; j < len(sortedPlayers); j += tableCount {
			tablePlayers = append(tablePlayers, sortedPlayers[j])
		}

		tableID := fmt.Sprintf("%s-table-%d", m.ID, i+1)

		// Create new table
		table := NewTable(m.app, m.ID, tableID, tablePlayers)
		if err := table.StartGame(m.Config); err != nil {
			logrus.Errorf("Failed to start game for table %s: %v", tableID, err)
			continue
		}
		m.Tables[tableID] = table

		// Notify game service to start game for this table
		gameReq := &sproto.Match2GameReq{
			StartGameReq: &sproto.StartGameReq{
				Matchid:      m.ID,
				Tableid:      tableID,
				Players:      tablePlayers,
				GameConfig:   m.Config.GameConfig.Rules,
				InitialChips: int32(m.Config.InitialChips),
				ScoreBase:    int32(m.Config.ScoreBase),
			},
		}

		rsp := &cproto.CommonResponse{Err: cproto.ErrCode_OK}
		if err := m.app.RPC(context.Background(), "game.match.message", rsp, gameReq); err != nil {
			logrus.Errorf("Failed to start mahjong game for table %s: %v", tableID, err)
			continue
		}

		// Notify players to enter this table
		startAck := &cproto.StartClientAck{
			Matchid: m.ID,
			Tableid: tableID,
			Players: tablePlayers,
		}

		if err := m.app.GroupBroadcast(context.Background(), "proxy", m.ID, "matchmsg", startAck); err != nil {
			logrus.Errorf("Failed to broadcast start ack for table %s: %v", tableID, err)
			continue
		}

		logrus.Infof("Mahjong table %s started with players: %v", tableID, tablePlayers)
	}
}
