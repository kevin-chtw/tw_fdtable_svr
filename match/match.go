package match

import (
	"context"
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
func (m *Match) HandleCreateRoom(ctx context.Context, req *cproto.CreateRoomReq) *cproto.CreateRoomAck {
	ack := &cproto.CreateRoomAck{}
	uid := m.app.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		ack.ErrorCode = 1 // 未登录
		return ack
	}

	tableId, err := m.tableIds.Take()
	if err != nil {
		ack.ErrorCode = 2 // 号已满，无法创建新桌子
		return ack
	}

	table := NewTable(m.app, m.ID, tableId)
	table.Init(uid, req.GameCount)

	m.tables.Store(tableId, table)
	ack.Tableid = tableId
	return ack
}

// 处理房卡加入请求
func (m *Match) HandleJoinRoom(ctx context.Context, req *cproto.JoinRoomReq) *cproto.JoinRoomAck {
	ack := &cproto.JoinRoomAck{}
	uid := m.app.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		ack.ErrorCode = 1 // 未登录
		return ack
	}

	table, ok := m.tables.Load(req.Tableid)
	if !ok {
		ack.ErrorCode = 3 // 桌子不存在
		return ack
	}
	table.(*Table).AddPlayer(uid)
	ack.Tableid = req.Tableid
	ack.ErrorCode = 0 // 成功
	return ack
}
