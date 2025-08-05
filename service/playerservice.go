package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/gogo/protobuf/proto"
	"github.com/kevin-chtw/tw_match_svr/match"
	"github.com/kevin-chtw/tw_proto/cproto"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
)

type handlerFunc func(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error)

type PlayerService struct {
	component.Base
	app      pitaya.Pitaya
	handlers map[reflect.Type]handlerFunc
}

func NewPlayerService(app pitaya.Pitaya) *PlayerService {
	p := &PlayerService{
		app:      app,
		handlers: make(map[reflect.Type]handlerFunc),
	}
	p.Init()
	return p
}

// extractMessage 使用反射从oneof接口中提取具体消息
func ExtractOneOf(oneof interface{}) (proto.Message, error) {
	if oneof == nil {
		return nil, errors.New("nil oneof value")
	}

	val := reflect.ValueOf(oneof)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// 获取包含具体消息的字段
	field := val.FieldByName("XXX_OneofWrappers")
	if !field.IsValid() {
		return nil, errors.New("not a oneof type")
	}

	// 获取具体消息值
	msgField := val.Field(0) // 具体消息总是第一个字段
	if msgField.IsNil() {
		return nil, errors.New("oneof contains nil message")
	}

	msg, ok := msgField.Interface().(proto.Message)
	if !ok {
		return nil, fmt.Errorf("field does not implement proto.Message: %T", msgField.Interface())
	}

	return msg, nil
}

func (p *PlayerService) Init() {
	// 注册具体消息类型的handler
	p.handlers[reflect.TypeOf(&cproto.CreateRoomReq{})] = p.handleCreateRoom
	p.handlers[reflect.TypeOf(&cproto.JoinRoomReq{})] = p.handleJoinRoom
}

func (p *PlayerService) Message(ctx context.Context, req *cproto.MatchReq) (*cproto.MatchAck, error) {
	if req == nil {
		return nil, errors.New("nil request")
	}

	// 获取oneof中的具体值
	v := req.Req
	if v == nil {
		return nil, errors.New("empty oneof")
	}

	// 使用反射提取具体消息
	msg, err := ExtractOneOf(v)
	if err != nil {
		return nil, fmt.Errorf("failed to extract message: %v", err)
	}

	// 查找对应的handler
	fn, ok := p.handlers[reflect.TypeOf(msg)]
	if !ok {
		return nil, fmt.Errorf("no handler for message type: %T", msg)
	}

	return fn(ctx, req)
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
