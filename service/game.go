package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/kevin-chtw/tw_common/utils"
	"github.com/kevin-chtw/tw_match_svr/match"
	"github.com/kevin-chtw/tw_proto/sproto"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
	"github.com/topfreegames/pitaya/v3/pkg/protos"
	"google.golang.org/protobuf/proto"
)

type Game struct {
	component.Base
	app      pitaya.Pitaya
	handlers map[string]func(*match.Match, int32, proto.Message) error
}

func NewGame(app pitaya.Pitaya) *Game {
	return &Game{
		app:      app,
		handlers: make(map[string]func(*match.Match, int32, proto.Message) error),
	}
}

func (g *Game) Init() {
	g.handlers[utils.TypeUrl(&sproto.GameResultAck{})] = (*match.Match).HandleGameResult
}

func (g *Game) Message(ctx context.Context, ack *sproto.Match2GameAck) (*protos.Response, error) {
	if ack == nil {
		return nil, errors.New("nil request: MatchReq cannot be nil")
	}

	logger.Log.Info(ack.String(), ack.Ack.TypeUrl)

	match := match.GetMatchManager().Get(ack.Matchid)
	if match == nil {
		return nil, fmt.Errorf("match not found for ID %d", ack.Matchid)
	}

	msg, err := ack.Ack.UnmarshalNew()
	if err != nil {
		return nil, err
	}

	if handler, ok := g.handlers[ack.Ack.TypeUrl]; ok {
		err := handler(match, ack.Tableid, msg)
		if err != nil {
			return nil, err
		}
		return &protos.Response{}, nil
	}

	return nil, errors.New("invalid request type")
}
