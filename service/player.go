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
	"google.golang.org/protobuf/types/known/anypb"
)

type Player struct {
	component.Base
	app      pitaya.Pitaya
	handlers map[string]func(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error)
}

func NewPlayerService(app pitaya.Pitaya) *Player {
	return &Player{
		app:      app,
		handlers: make(map[string]func(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error)),
	}
}

func (p *Player) Init() {
	p.handlers[utils.TypeUrl(&cproto.CreateRoomReq{})] = p.handleCreateRoom
	p.handlers[utils.TypeUrl(&cproto.JoinRoomReq{})] = p.handleJoinRoom
}

func (p *Player) Message(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error) {
	if req == nil {
		return nil, errors.New("nil request: MatchReq cannot be nil")
	}

	logger.Log.Info(req.String(), req.Req.TypeUrl)
	if handler, ok := p.handlers[req.Req.TypeUrl]; ok {
		return handler(ctx, req)
	}

	return nil, errors.New("invalid request type")
}

func (p *Player) Online(ctx context.Context, req *sproto.Proxy2MatchReq) (*sproto.Proxy2MatchAck, error) {
	logger.Log.Info("PlayerService::Online")
	return nil, nil
}

func (p *Player) Offline(ctx context.Context, req *sproto.Proxy2MatchReq) (*sproto.Proxy2MatchAck, error) {
	logger.Log.Info("PlayerService::offline")
	return nil, nil
}

func (p *Player) newMatchAck(req *cproto.MatchReq, ack proto.Message) (*cproto.MatchAck, error) {
	if ack == nil {
		return nil, errors.New("failed to create room")
	}

	data, err := anypb.New(ack)
	if err != nil {
		return nil, err
	}

	return &cproto.MatchAck{
		Serverid: p.app.GetServerID(),
		Matchid:  req.GetMatchid(),
		Ack:      data,
	}, nil
}

func (p *Player) handleCreateRoom(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error) {
	msg := &cproto.CreateRoomReq{}
	if err := proto.Unmarshal(req.Req.Value, msg); err != nil {
		return nil, err
	}

	match := match.GetMatchManager().Get(req.Matchid)
	if match == nil {
		return nil, fmt.Errorf("match not found for ID %d", req.Matchid)
	}
	createAck, err := match.HandleCreateRoom(ctx, msg)
	if err != nil {
		return nil, err
	}
	return p.newMatchAck(req, createAck)
}

func (p *Player) handleJoinRoom(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error) {
	msg := &cproto.JoinRoomReq{}
	if err := proto.Unmarshal(req.Req.Value, msg); err != nil {
		return nil, err
	}

	match := match.GetMatchManager().Get(req.Matchid)
	if match == nil {
		return nil, fmt.Errorf("match not found for ID %d", req.Matchid)
	}
	joinAck, err := match.HandleJoinRoom(ctx, msg)
	if err != nil {
		return nil, err
	}
	return p.newMatchAck(req, joinAck)
}
