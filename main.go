package main

import (
	"strings"

	"github.com/kevin-chtw/tw_match_svr/service"
	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
	"github.com/topfreegames/pitaya/v3/pkg/config"
	"github.com/topfreegames/pitaya/v3/pkg/modules"
)

var app pitaya.Pitaya

func main() {
	serverType := "match"
	logrus.SetLevel(logrus.DebugLevel)

	config := config.NewDefaultPitayaConfig()
	builder := pitaya.NewDefaultBuilder(false, serverType, pitaya.Cluster, map[string]string{}, *config)
	app = builder.Build()

	defer app.Shutdown()
	bs := modules.NewETCDBindingStorage(builder.Server, builder.SessionPool, builder.Config.Modules.BindingStorage.Etcd)
	app.RegisterModule(bs, "matchingstorage")

	logrus.Infof("Pitaya server of type %s started", serverType)
	initServices()
	app.Start()
}

func initServices() {
	playersvc := service.NewPlayerService(app)
	app.Register(playersvc, component.WithName("player"), component.WithNameFunc(strings.ToLower))
	app.RegisterRemote(playersvc, component.WithName("player"), component.WithNameFunc(strings.ToLower))
}
