package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/kevin-chtw/tw_common/matchbase"
	"github.com/kevin-chtw/tw_common/utils"
	"github.com/kevin-chtw/tw_fdtable_svr/match"
	"github.com/kevin-chtw/tw_proto/sproto"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
	"google.golang.org/protobuf/proto"
)

type Remote struct {
	component.Base
	app      pitaya.Pitaya
	handlers map[string]func(*match.Match, proto.Message) error
}

func NewRemote(app pitaya.Pitaya) *Remote {
	return &Remote{
		app:      app,
		handlers: make(map[string]func(*match.Match, proto.Message) error),
	}
}

func (g *Remote) Init() {
	g.handlers[utils.TypeUrl(&sproto.NetStateReq{})] = (*match.Match).HandleNetState
	g.handlers[utils.TypeUrl(&sproto.GameResultReq{})] = (*match.Match).HandleGameResult
	g.handlers[utils.TypeUrl(&sproto.GameOverReq{})] = (*match.Match).HandleGameOver
}

func (g *Remote) Message(ctx context.Context, req *sproto.MatchReq) (*sproto.MatchAck, error) {
	if req == nil {
		return nil, errors.New("nil request: MatchReq cannot be nil")
	}

	logger.Log.Info(req.String(), req.Req.TypeUrl)

	msg, err := req.Req.UnmarshalNew()
	if err != nil {
		return nil, err
	}

	base := matchbase.GetMatch(req.Matchid)
	if base == nil {
		return nil, fmt.Errorf("match not found for ID %d", req.Matchid)
	}
	m := base.Sub.(*match.Match)
	if handler, ok := g.handlers[req.Req.TypeUrl]; ok {
		err := handler(m, msg)
		if err != nil {
			return nil, err
		}
		return &sproto.MatchAck{}, nil
	}

	return nil, errors.New("invalid request type")
}
