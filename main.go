package main

import (
	"strings"

	"github.com/kevin-chtw/tw_common/matchbase"
	"github.com/kevin-chtw/tw_common/storage"
	"github.com/kevin-chtw/tw_common/utils"
	"github.com/kevin-chtw/tw_fdtable_svr/match"
	"github.com/kevin-chtw/tw_fdtable_svr/service"
	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
	"github.com/topfreegames/pitaya/v3/pkg/config"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
	"github.com/topfreegames/pitaya/v3/pkg/serialize"
)

var app pitaya.Pitaya

func main() {
	serverType := "fdtable"
	pitaya.SetLogger(utils.Logger(logrus.InfoLevel))

	config := config.NewDefaultPitayaConfig()
	config.SerializerType = uint16(serialize.PROTOBUF)
	config.Handler.Messages.Compression = false
	builder := pitaya.NewDefaultBuilder(false, serverType, pitaya.Cluster, map[string]string{}, *config)
	app = builder.Build()

	defer app.Shutdown()
	bs := storage.NewETCDMatching(builder.Server, builder.Config.Modules.BindingStorage.Etcd)
	app.RegisterModule(bs, "matchingstorage")

	logger.Log.Infof("Pitaya server of type %s started", serverType)
	initServices()
	matchbase.Init(app, match.NewMatch, match.NewTable, match.NewPlayer)

	app.Start()
}

func initServices() {
	player := service.NewPlayer(app)
	app.Register(player, component.WithName("player"), component.WithNameFunc(strings.ToLower))

	remote := service.NewRemote(app)
	app.RegisterRemote(remote, component.WithName("remote"), component.WithNameFunc(strings.ToLower))
}
