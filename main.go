package main

import (
	"strings"

	"github.com/kevin-chtw/tw_common/storage"
	"github.com/kevin-chtw/tw_common/utils"
	"github.com/kevin-chtw/tw_match_svr/match"
	"github.com/kevin-chtw/tw_match_svr/service"
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
	builder := pitaya.NewDefaultBuilder(false, serverType, pitaya.Cluster, map[string]string{}, *config)
	app = builder.Build()

	defer app.Shutdown()
	bs := storage.NewETCDMatching(builder.Server, builder.Config.Modules.BindingStorage.Etcd)
	app.RegisterModule(bs, "matchingstorage")

	logger.Log.Infof("Pitaya server of type %s started", serverType)
	initServices()
	match.InitGame(app)

	app.Start()
}

func initServices() {
	playersvc := service.NewPlayerService(app)
	app.Register(playersvc, component.WithName("player"), component.WithNameFunc(strings.ToLower))
	app.RegisterRemote(playersvc, component.WithName("player"), component.WithNameFunc(strings.ToLower))
}
