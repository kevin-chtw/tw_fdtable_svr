package match

import (
	"context"

	"github.com/kevin-chtw/tw_common/matchbase"
)

type player struct {
	*matchbase.Player
}

func NewPlayer(ctx context.Context, id string, matchId int32, score int64) *matchbase.Player {
	p := &player{}
	return matchbase.NewPlayer(p, ctx, id, matchId, score)
}
