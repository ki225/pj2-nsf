package pkg

import (
	"context"
	"fmt"
	"nsf/internal/sbi"
	"nsf/logger"
	"nsf/model"

	loggergoUtil "github.com/Alonza0314/logger-go/v2/util"
	"github.com/davecgh/go-spew/spew"
	"github.com/free-ran-ue/util"
)

type App struct {
	*model.NsfConfig

	*sbi.Server

	logger.NsfLogger
}

func NewApp(configFilePath string) (*App, error) {
	var config model.NsfConfig
	if err := util.LoadFromYaml(configFilePath, &config); err != nil {
		return nil, err
	}

	logger := logger.NewNsfLogger(loggergoUtil.LogLevelString(config.Logger.Level))
	server := sbi.NewServer(&sbi.ServerIE{
		SbiAddress: fmt.Sprintf("%s:%d", config.Nsf.SbiAddress, config.Nsf.SbiPort),

		DbType:  config.Nsf.DbType,
		MongoDB: config.Nsf.MongoDB,

		NsfLogger: &logger,
	})

	return &App{
		NsfConfig: &config,

		Server: server,

		NsfLogger: logger,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	a.NsfLogger.AppLog.Infoln("Starting App server...")

	a.printConfig()

	if err := a.Server.Run(ctx); err != nil {
		return err
	}

	a.NsfLogger.AppLog.Infoln("App server started")
	return nil
}

func (a *App) ShutDown() {
	a.NsfLogger.AppLog.Infoln("Shutting down App server...")

	a.Server.ShutDown()

	a.NsfLogger.AppLog.Infoln("App server shut down")
}

func (a *App) printConfig() {
	scs := &spew.ConfigState{
		Indent:                  "    ",
		DisablePointerAddresses: true,
		DisableCapacities:       true,
		SortKeys:                false,
	}
	a.NsfLogger.AppLog.Infof("App Config:\n%s", scs.Sdump(a.NsfConfig))
}
