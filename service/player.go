package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/kevin-chtw/tw_common/utils"
	"github.com/kevin-chtw/tw_match_svr/match"
	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
	"google.golang.org/protobuf/proto"
)

type Player struct {
	component.Base
	app      pitaya.Pitaya
	handlers map[string]func(*match.Match, context.Context, proto.Message) (proto.Message, error)
}

func NewPlayer(app pitaya.Pitaya) *Player {
	return &Player{
		app:      app,
		handlers: make(map[string]func(*match.Match, context.Context, proto.Message) (proto.Message, error)),
	}
}

func (p *Player) Init() {
	p.handlers[utils.TypeUrl(&cproto.CreateRoomReq{})] = (*match.Match).HandleCreateRoom
	p.handlers[utils.TypeUrl(&cproto.JoinRoomReq{})] = (*match.Match).HandleJoinRoom
	p.handlers[utils.TypeUrl(&cproto.CancelRoomReq{})] = (*match.Match).HandleCancelRoom
}

func (p *Player) Message(ctx context.Context, data []byte) ([]byte, error) {
	req := &cproto.MatchReq{}
	if err := utils.Unmarshal(ctx, data, req); err != nil {
		return nil, err
	}

	logger.Log.Info(req.String(), req.Req.TypeUrl)

	match := match.GetMatchManager().Get(req.Matchid)
	if match == nil {
		return nil, fmt.Errorf("match not found for ID %d", req.Matchid)
	}
	msg, err := req.Req.UnmarshalNew()
	if err != nil {
		return nil, err
	}

	if handler, ok := p.handlers[req.Req.TypeUrl]; ok {
		if rsp, err := handler(match, ctx, msg); err != nil {
			return nil, err
		} else {
			return match.NewMatchAck(ctx, rsp)
		}
	}

	return nil, errors.ErrUnsupported
}

func (p *Player) Net(ctx context.Context, req *sproto.NetStateReq) (*sproto.NetStateAck, error) {
	uid := req.GetUid()
	player, err := match.GetPlayerManager().Load(uid)
	if err != nil {
		return nil, err
	}
	return &sproto.NetStateAck{}, player.NetChange(req.GetOnline())
}
