package main

import (
	"strings"

	"github.com/kevin-chtw/tw_match_svr/service"
	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
	"github.com/topfreegames/pitaya/v3/pkg/config"
)

var app pitaya.Pitaya

func main() {
	serverType := "match"

	logrus.SetLevel(logrus.DebugLevel)

	config := config.NewDefaultPitayaConfig()
	builder := pitaya.NewDefaultBuilder(false, serverType, pitaya.Cluster, map[string]string{}, *config)
	app = builder.Build()

	defer app.Shutdown()

	logrus.Infof("Pitaya server of type %s started", serverType)
	initServices()
	app.Start()
}

func initServices() {
	matchsvc := service.NewMatchService(app)
	app.Register(matchsvc, component.WithName("match"), component.WithNameFunc(strings.ToLower))
	app.RegisterRemote(matchsvc, component.WithName("match"), component.WithNameFunc(strings.ToLower))
}
