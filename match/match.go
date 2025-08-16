package match

import (
	"context"
	"errors"
	"sync"

	"github.com/kevin-chtw/tw_proto/cproto"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
)

// Table定义已移动到table.go

type Match struct {
	ID        int32
	app       pitaya.Pitaya
	tables    sync.Map
	config    MatchConfig
	matchType int // 0: 普通匹配, 1: 房卡模式
	tableIds  *TableIDs
}

func NewMatch(app pitaya.Pitaya, id int32, config MatchConfig) *Match {
	return &Match{
		app:       app,
		ID:        id,
		tables:    sync.Map{},
		config:    config,
		matchType: 1, // 默认普通匹配模式
		tableIds:  NewTableIDs(),
	}
}

// 处理房卡创建请求
func (m *Match) HandleCreateRoom(ctx context.Context, req *cproto.CreateRoomReq) (*cproto.CreateRoomAck, error) {
	uid := m.app.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		return nil, errors.New("未登录")
	}

	player := playerManager.LoadOrStore(uid)
	if player == nil {
		return nil, errors.New("玩家不存在")
	}

	if player.InRoom {
		return nil, errors.New("玩家已在游戏中")
	}

	tableId, err := m.tableIds.Take()
	if err != nil {
		return nil, errors.New("桌号已满")
	}

	table := NewTable(m.app, m.ID, tableId)
	table.Init(player, req.GameCount)

	m.tables.Store(tableId, table)
	return &cproto.CreateRoomAck{Tableid: tableId}, nil
}

// 处理房卡加入请求
func (m *Match) HandleJoinRoom(ctx context.Context, req *cproto.JoinRoomReq) (*cproto.JoinRoomAck, error) {
	uid := m.app.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		return nil, errors.New("未登录")
	}

	table, ok := m.tables.Load(req.Tableid)
	if !ok {
		return nil, errors.New("桌子不存在")
	}

	player := playerManager.LoadOrStore(uid)
	if player == nil {
		return nil, errors.New("玩家不存在")
	}

	if player.InRoom {
		return nil, errors.New("玩家已在游戏中")
	}
	table.(*Table).AddPlayer(player)
	return &cproto.JoinRoomAck{Tableid: req.Tableid}, nil
}
