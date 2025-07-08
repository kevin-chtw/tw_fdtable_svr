package service

import (
	"context"
	"errors"

	"github.com/kevin-chtw/tw_match_svr/match"
	"github.com/kevin-chtw/tw_proto/cproto"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
)

// PlayerService is a service that manages matches in a game.
type PlayerService struct {
	component.Base
	app pitaya.Pitaya
}

// NewPlayerService creates a new instance of PlayerService.
func NewPlayerService(app pitaya.Pitaya) *PlayerService {
	return &PlayerService{
		app: app,
	}
}

func (p *PlayerService) Message(ctx context.Context, req *cproto.MatchReq) (*cproto.CommonResponse, error) {
	userID := p.app.GetSessionFromCtx(ctx).UID()
	if userID == "" {
		return nil, errors.New("user ID is empty")
	}

	player := match.GetPlayerManager().LoadOrStore(userID)
	player.HandleMessage(ctx, req)
	return &cproto.CommonResponse{
		Err: 0,
	}, nil
}
