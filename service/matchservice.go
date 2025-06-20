package service

import (
	"context"
	"strconv"

	"github.com/kevin-chtw/tw_proto/cproto"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
)

var sequence int

// MatchService is a service that manages matches in a game.
type MatchService struct {
	component.Base
	app    pitaya.Pitaya
	matchs map[string]*Match
}

func NewMatchService(app pitaya.Pitaya) *MatchService {
	return &MatchService{
		app:    app,
		matchs: make(map[string]*Match),
	}
}

func (ms *MatchService) AfterInit() {
	sequence++
	matchId := strconv.Itoa(sequence)
	ms.matchs[matchId] = NewMatch(ms.app, matchId)
}

func (ms *MatchService) Sign(ctx context.Context, req *cproto.SignupReq) (*cproto.CommonResponse, error) {
	match, ok := ms.matchs[req.Matchid]
	if !ok {
		return &cproto.CommonResponse{
			Err: cproto.ErrCode_ERR}, nil
	}

	if !match.AddPlayer(ctx) {
		return &cproto.CommonResponse{
			Err: cproto.ErrCode_ERR,
		}, nil
	}

	return &cproto.CommonResponse{
		Err: cproto.ErrCode_OK,
	}, nil
}
