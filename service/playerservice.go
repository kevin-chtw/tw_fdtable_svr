package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/kevin-chtw/tw_match_svr/match"
	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
)

type PlayerService struct {
	component.Base
	app      pitaya.Pitaya
	handlers map[reflect.Type]func(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error)
}

func NewPlayerService(app pitaya.Pitaya) *PlayerService {
	return &PlayerService{
		app:      app,
		handlers: make(map[reflect.Type]func(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error)),
	}
}

func (p *PlayerService) Init() {
	logrus.Info("PlayerService initialized")
	p.handlers[reflect.TypeOf(&cproto.MatchReq_CreateRoomReq{})] = p.handleCreateRoom
	p.handlers[reflect.TypeOf(&cproto.MatchReq_JoinRoomReq{})] = p.handleJoinRoom
}

func (p *PlayerService) Message(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error) {
	if req == nil {
		return nil, errors.New("nil request")
	}

	logrus.Info(req)
	if req.Req == nil {
		return nil, errors.New("empty oneof")
	}

	// 查找对应的handler
	fn, ok := p.handlers[reflect.TypeOf(req.Req)]
	if !ok {
		return nil, fmt.Errorf("no handler for message type: %T", req.Req)
	}

	return fn(ctx, req)
}

func (p *PlayerService) Online(ctx context.Context, req *sproto.Proxy2MatchReq) (*sproto.Proxy2MatchAck, error) {
	logrus.Info("PlayerService::Online")
	return nil, nil
}

func (p *PlayerService) Offline(ctx context.Context, req *sproto.Proxy2MatchReq) (*sproto.Proxy2MatchAck, error) {
	logrus.Info("PlayerService::offline")
	return nil, nil
}

func (p *PlayerService) NewMatchAck(req *cproto.MatchReq) *cproto.MatchAck {
	return &cproto.MatchAck{
		Serverid: p.app.GetServerID(),
		Matchid:  req.GetMatchid(),
	}
}

func (p *PlayerService) handleCreateRoom(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error) {
	msg := req.GetCreateRoomReq()
	if nil == msg {
		return nil, errors.New("invalid request type")
	}

	match := match.GetMatchManager().Get(req.Matchid)
	if match == nil {
		return nil, fmt.Errorf("match not found for ID %d", req.Matchid)
	}
	createAck := match.HandleCreateRoom(ctx, msg)
	if createAck == nil {
		return nil, errors.New("failed to create room")
	}

	ack := p.NewMatchAck(req)
	ack.Ack = &cproto.MatchAck_CreateRoomAck{CreateRoomAck: createAck}
	return ack, nil
}

func (p *PlayerService) handleJoinRoom(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error) {
	msg := req.GetJoinRoomReq()
	if nil == msg {
		return nil, errors.New("invalid request type")
	}

	match := match.GetMatchManager().Get(req.Matchid)
	if match == nil {
		return nil, fmt.Errorf("match not found for ID %d", req.Matchid)
	}
	joinAck := match.HandleJoinRoom(ctx, msg)
	if joinAck == nil {
		return nil, errors.New("failed to join room")
	}

	ack := p.NewMatchAck(req)
	ack.Ack = &cproto.MatchAck_JoinRoomAck{JoinRoomAck: joinAck}
	return ack, nil
}
