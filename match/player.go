package match

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/kevin-chtw/tw_proto/cproto"
)

// PlayerState 定义玩家状态常量
type PlayerState string

const (
	StateReady   PlayerState = "ready"
	StateWaiting PlayerState = "waiting"
	StatePlaying PlayerState = "playing"
	StateOffline PlayerState = "offline"
)

// Player 表示游戏中的玩家
type Player struct {
	ID       string      // 玩家唯一ID
	SeatNum  int         // 座位号
	Score    int32       // 玩家积分
	Status   PlayerState // 玩家状态
	mu       sync.RWMutex
	handlers map[reflect.Type]func(context.Context, interface{}) *cproto.MatchAck
	joinTime time.Time // 玩家加入时间
}

// initHandlers 初始化请求处理器映射
func (p *Player) initHandlers() {
	p.handlers = map[reflect.Type]func(context.Context, interface{}) *cproto.MatchAck{
		reflect.TypeOf((*cproto.SignupReq)(nil)):      p.handleSignup,
		reflect.TypeOf((*cproto.SignoutReq)(nil)):     p.handleSignout,
		reflect.TypeOf((*cproto.EnterMatchReq)(nil)):  p.handleEnterMatch,
		reflect.TypeOf((*cproto.MatchStatusReq)(nil)): p.handleMatchStatus,
	}
}

// NewPlayer 创建新玩家实例
func NewPlayer(id string) *Player {
	p := &Player{
		ID:       id,
		Status:   StateReady,
		joinTime: time.Now(),
	}
	p.initHandlers()
	return p
}

// HandleMessage 处理玩家消息
func (p *Player) HandleMessage(ctx context.Context, req *cproto.MatchReq) *cproto.MatchAck {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 获取请求的具体类型
	reqType := reflect.TypeOf(req.Req)

	// 从映射表获取处理函数
	if handler, exists := p.handlers[reqType]; exists {
		// 提取请求数据
		reqData := reflect.ValueOf(req.Req).Elem().Field(0).Interface()
		return handler(ctx, reqData)
	}

	// 未知请求类型
	ack := &cproto.MatchAck{Serverid: req.GetServerid()}
	ack.Ack = &cproto.MatchAck_SingupAck{
		SingupAck: &cproto.SingupAck{ErrorCode: 400},
	}
	return ack
}

// 以下是各处理方法的实现（方法名需与请求类型对应）
func (p *Player) handleSignup(ctx context.Context, req interface{}) *cproto.MatchAck {
	signupReq, ok := req.(*cproto.SignupReq)
	if !ok {
		return &cproto.MatchAck{
			Ack: &cproto.MatchAck_SingupAck{
				SingupAck: &cproto.SingupAck{ErrorCode: 400},
			},
		}
	}

	// 实际业务逻辑：将玩家添加到匹配队列
	// 这里需要调用匹配系统的API进行实际处理
	// 示例代码，实际实现需要根据项目架构调整
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Status != StateReady {
		p.logWithContext(ctx, "WARN", "player already signed up")
		return p.createSignupAck("", 2) // 玩家已报名
	}

	p.Status = StateWaiting
	p.logWithContext(ctx, "INFO", "player signed up successfully", "matchid", signupReq.Matchid)
	return &cproto.MatchAck{
		Ack: &cproto.MatchAck_SingupAck{
			SingupAck: &cproto.SingupAck{
				Matchid:   signupReq.Matchid,
				ErrorCode: 0, // 成功
			},
		},
	}
}

// 创建响应对象的辅助函数
func (p *Player) createSignupAck(matchid string, errorCode int32) *cproto.MatchAck {
	ack := &cproto.MatchAck{
		Ack: &cproto.MatchAck_SingupAck{
			SingupAck: &cproto.SingupAck{
				Matchid:   matchid,
				ErrorCode: errorCode,
			},
		},
	}

	// 添加调试信息
	if errorCode != 0 {
		p.debugLog("createSignupAck error", "matchid", matchid, "errorCode", errorCode)
	}
	return ack
}

func (p *Player) createEnterMatchAck(matchid, tableid string, players []string, errorCode int32) *cproto.MatchAck {
	return &cproto.MatchAck{
		Ack: &cproto.MatchAck_EnterMatchAck{
			EnterMatchAck: &cproto.EnterMatchAck{
				Matchid:   matchid,
				Tableid:   tableid,
				Players:   players,
				ErrorCode: errorCode,
			},
		},
	}
}

// logWithContext 带上下文的日志记录
func (p *Player) logWithContext(ctx context.Context, level, msg string, fields ...interface{}) {
	// 实际项目中应该使用日志库如logrus/zap
	// 这里简化实现仅用于演示
	logFields := []interface{}{
		"playerID", p.ID,
		"status", p.Status,
	}
	logFields = append(logFields, fields...)

	if ctx != nil {
		if reqID := ctx.Value("requestID"); reqID != nil {
			logFields = append(logFields, "requestID", reqID)
		}
	}

	println(level+":", msg, logFields)
}

func (p *Player) handleSignout(ctx context.Context, req interface{}) *cproto.MatchAck {
	signoutReq, ok := req.(*cproto.SignoutReq)
	if !ok {
		return p.createSignupAck("", 400)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Status != StateWaiting {
		p.logWithContext(ctx, "WARN", "player not in waiting state")
		return p.createSignupAck("", 2) // 玩家未报名
	}

	p.Status = StateReady
	p.logWithContext(ctx, "INFO", "player signed out successfully", "matchid", signoutReq.Matchid)
	return p.createSignupAck(signoutReq.Matchid, 0) // 成功
}

func (p *Player) handleEnterMatch(ctx context.Context, req interface{}) *cproto.MatchAck {
	enterReq, ok := req.(*cproto.EnterMatchReq)
	if !ok {
		return &cproto.MatchAck{
			Ack: &cproto.MatchAck_SingupAck{
				SingupAck: &cproto.SingupAck{ErrorCode: 400},
			},
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Status != StateWaiting {
		p.logWithContext(ctx, "WARN", "player not in waiting state")
		return p.createEnterMatchAck("", "", nil, 2) // 玩家未报名
	}

	p.Status = StatePlaying
	p.logWithContext(ctx, "INFO", "player entered match",
		"matchid", enterReq.Matchid,
		"tableid", enterReq.Tableid)
	return &cproto.MatchAck{
		Ack: &cproto.MatchAck_EnterMatchAck{
			EnterMatchAck: &cproto.EnterMatchAck{
				Matchid:   enterReq.Matchid,
				Tableid:   enterReq.Tableid,
				Players:   []string{p.ID}, // 示例数据，实际应从匹配系统获取
				ErrorCode: 0,              // 成功
			},
		},
	}
}

func (p *Player) handleMatchStatus(ctx context.Context, req interface{}) *cproto.MatchAck {
	statusReq, ok := req.(*cproto.MatchStatusReq)
	if !ok {
		return &cproto.MatchAck{
			Ack: &cproto.MatchAck_SingupAck{
				SingupAck: &cproto.SingupAck{ErrorCode: 400},
			},
		}
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// 示例数据，实际应从匹配系统获取
	return &cproto.MatchAck{
		Ack: &cproto.MatchAck_StatusAck{
			StatusAck: &cproto.MatchStatusAck{
				Matchid: statusReq.Matchid,
				Status:  1, // 1: matching
				Players: []string{p.ID},
				Timeout: 60, // 60秒超时
				Tables: []*cproto.TableInfo{
					{
						Tableid:  "table1",
						Players:  []string{p.ID},
						Status:   0, // waiting
						CreateAt: time.Now().Unix(),
					},
				},
			},
		},
	}
}
