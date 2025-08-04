package match

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
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
	mu           sync.RWMutex

	// 房卡模式特有字段
	MatchType  int    // 0: 普通匹配, 1: 房卡模式
	Creator    string // 房主UID
	InviteCode string // 邀请码
	Password   string // 房间密码
	RoundCount int    // 总局数
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
		MatchType:    0, // 默认普通匹配模式
	}

	return match
}

func NewRoomCardMatch(app pitaya.Pitaya, id string, config MatchConfig, creator string, inviteCode string, password string, roundCount int) *Match {
	match := NewMatch(app, id, config)
	match.MatchType = 1 // 房卡模式
	match.Creator = creator
	match.InviteCode = inviteCode
	match.Password = password
	match.RoundCount = roundCount
	return match
}

func (m *Match) HandleMatchReq(ctx context.Context, req *cproto.MatchReq) *cproto.MatchAck {
	ack := &cproto.MatchAck{
		Serverid: m.app.GetServerID(),
	}

	switch {
	case req.GetCreateRoomReq() != nil:
		createAck := m.HandleCreateRoom(ctx, req.GetCreateRoomReq())
		ack.Ack = &cproto.MatchAck_CreateRoomAck{CreateRoomAck: createAck}
	case req.GetJoinRoomReq() != nil:
		joinAck := m.HandleRoomCardJoin(ctx, req.GetJoinRoomReq())
		ack.Ack = &cproto.MatchAck_JoinRoomAck{JoinRoomAck: joinAck}
	}
	return ack
}

// 处理房卡创建请求
func (m *Match) HandleCreateRoom(ctx context.Context, req *cproto.CreateRoomReq) *cproto.CreateRoomAck {
	uid := m.app.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		return &cproto.CreateRoomAck{
			ErrorCode: 2, // 创建失败
		}
	}

	// 检查房卡数量
	checkReq := &sproto.CheckRoomCardsReq{Userid: uid}
	checkAck := &sproto.CheckRoomCardsAck{}
	err := m.app.RPC(ctx, "db.player.checkroomcards", checkReq, checkAck)
	if err != nil {
		logrus.Errorf("Failed to check room cards: %v", err)
		return &cproto.RoomCardCreateAck{
			ErrorCode: 2, // 创建失败
		}
	}

	if checkAck.Count <= 0 {
		return &cproto.RoomCardCreateAck{
			ErrorCode: 1, // 房卡不足
		}
	}

	// 扣除房卡
	deductReq := &sproto.DeductRoomCardsReq{
		Userid: uid,
		Count:  1, // 每次创建扣除1张房卡
	}
	deductAck := &sproto.DeductRoomCardsAck{}
	err = m.app.RPC(ctx, "db.player.deductroomcards", deductReq, deductAck)
	if err != nil {
		logrus.Errorf("Failed to deduct room cards: %v", err)
		return &cproto.RoomCardCreateAck{
			ErrorCode: 2, // 创建失败
		}
	}

	// 生成邀请码
	inviteCode := generateInviteCode()

	// 修改匹配配置
	config := m.Config
	config.MaxPlayers = int(req.MaxPlayers)
	// 创建房卡模式匹配
	match := NewRoomCardMatch(
		m.app,
		generateMatchID(),
		config,
		uid,
		inviteCode,
		req.Password,
		int(req.RoundCount),
	)

	// 保存匹配到管理器
	matchManager := GetMatchManager()
	matchManager.AddMatch(match)

	return &cproto.RoomCardCreateAck{
		Matchid:    match.ID,
		ErrorCode:  0,
		InviteCode: inviteCode,
	}
}

// 处理房卡加入请求
func (m *Match) HandleRoomCardJoin(ctx context.Context, req *cproto.RoomCardJoinReq) *cproto.RoomCardJoinAck {
	// 通过邀请码获取匹配实例
	matchManager := GetMatchManager()
	match, err := matchManager.GetMatchByInviteCode(req.InviteCode)
	if err != nil {
		return &cproto.RoomCardJoinAck{
			ErrorCode: 1, // 房间不存在
		}
	}

	// 检查密码
	if match.Password != "" && match.Password != req.Password {
		return &cproto.RoomCardJoinAck{
			ErrorCode: 2, // 密码错误
		}
	}

	// 检查房间是否已满
	if len(match.PlayerIDs) >= match.Config.MaxPlayers {
		return &cproto.RoomCardJoinAck{
			ErrorCode: 3, // 房间已满
		}
	}

	// 加入房间
	uid := m.app.GetSessionFromCtx(ctx).UID()
	match.PlayerIDs = append(match.PlayerIDs, uid)
	match.ReadyPlayers[uid] = false

	return &cproto.RoomCardJoinAck{
		Matchid:        match.ID,
		ErrorCode:      0,
		Creator:        match.Creator,
		CurrentPlayers: int32(len(match.PlayerIDs)),
		MaxPlayers:     int32(match.Config.MaxPlayers),
	}
}

// 生成邀请码
func generateInviteCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// 生成匹配ID
func generateMatchID() string {
	return fmt.Sprintf("match-%d", time.Now().UnixNano())
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

	if m.MatchType == 1 {
		logrus.Infof("Starting room card mahjong match %s with %d players, creator: %s",
			m.ID, len(m.PlayerIDs), m.Creator)
	} else {
		logrus.Infof("Starting normal mahjong match %s with %d players", m.ID, len(m.PlayerIDs))
	}

	// Distribute players to tables with skill-based matching
	tableCount := len(m.PlayerIDs) / m.Config.MaxPlayers
	if len(m.PlayerIDs)%m.Config.MaxPlayers != 0 {
		tableCount++
	}

	// 房卡模式不排序玩家，保持加入顺序
	sortedPlayers := make([]string, len(m.PlayerIDs))
	copy(sortedPlayers, m.PlayerIDs)

	if m.MatchType == 0 { // 普通匹配才按技能排序
		sort.Slice(sortedPlayers, func(i, j int) bool {
			return sortedPlayers[i] < sortedPlayers[j] // Replace with actual skill comparison
		})
	}

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
				MatchType:    int32(m.MatchType),
				RoundCount:   int32(m.RoundCount),
			},
		}

		rsp := &cproto.CommonResponse{Err: cproto.ErrCode_OK}
		if err := m.app.RPC(context.Background(), "game.match.message", gameReq, rsp); err != nil {
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
