package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/kevin-chtw/tw_match_svr/match"
	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type PlayerService struct {
	component.Base
	app      pitaya.Pitaya
	handlers map[string]func(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error)
}

func NewPlayerService(app pitaya.Pitaya) *PlayerService {
	return &PlayerService{
		app:      app,
		handlers: make(map[string]func(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error)),
	}
}

func TypeUrl(src proto.Message) string {
	any, err := anypb.New(&cproto.CreateRoomReq{})
	if err != nil {
		logrus.Error(err)
		return ""
	}
	return any.GetTypeUrl()
}

func (p *PlayerService) Init() {
	logrus.Info("PlayerService initialized")

	p.handlers[TypeUrl(&cproto.CreateRoomReq{})] = p.handleCreateRoom
	p.handlers[TypeUrl(&cproto.JoinRoomReq{})] = p.handleJoinRoom
}

func (p *PlayerService) Message(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error) {
	if req == nil {
		return nil, errors.New("nil request: MatchReq cannot be nil")
	}

	if handler, ok := p.handlers[req.Req.TypeUrl]; ok {
		return handler(ctx, req)
	}

	return nil, errors.New("invalid request type")
}

func (p *PlayerService) Online(ctx context.Context, req *sproto.Proxy2MatchReq) (*sproto.Proxy2MatchAck, error) {
	logrus.Info("PlayerService::Online")
	return nil, nil
}

func (p *PlayerService) Offline(ctx context.Context, req *sproto.Proxy2MatchReq) (*sproto.Proxy2MatchAck, error) {
	logrus.Info("PlayerService::offline")
	return nil, nil
}

func (p *PlayerService) newMatchAck(req *cproto.MatchReq, ack proto.Message) (*cproto.MatchAck, error) {
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

func (p *PlayerService) handleCreateRoom(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error) {
	msg := &cproto.CreateRoomReq{}
	if err := proto.Unmarshal(req.Req.Value, msg); err != nil {
		return nil, err
	}

	match := match.GetMatchManager().Get(req.Matchid)
	if match == nil {
		return nil, fmt.Errorf("match not found for ID %d", req.Matchid)
	}
	createAck := match.HandleCreateRoom(ctx, msg)
	return p.newMatchAck(req, createAck)
}

func (p *PlayerService) handleJoinRoom(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error) {
	msg := &cproto.JoinRoomReq{}
	if err := proto.Unmarshal(req.Req.Value, msg); err != nil {
		return nil, err
	}

	match := match.GetMatchManager().Get(req.Matchid)
	if match == nil {
		return nil, fmt.Errorf("match not found for ID %d", req.Matchid)
	}
	joinAck := match.HandleJoinRoom(ctx, msg)
	return p.newMatchAck(req, joinAck)
}
