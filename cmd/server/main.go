package main

import (
	"context"
	"errors"
	"esptrans/pkg/api"
	"esptrans/pkg/config"
	"esptrans/pkg/favorites"
	"esptrans/pkg/translate"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

type App struct {
	db    *favorites.DBService
	trSvc *translate.Translate
}

func main() {
	cfg := &config.AppSettings{
		CommonConfig: config.CommonConfig{
			LibreTranslateURL: config.DefaultLibreTranslateURL,
			FavoritesDBURL:    "",
		},
		APIConfig: config.APIConfig{
			ListenAddr:  "localhost:8080",
			StaticPages: "./views",
		},
	}

	err := config.ReadConfig(cfg)
	if err != nil {
		logrus.Fatal(err)
	}

	app := &App{}

	if cfg.FavoritesDBURL != "" {
		// open the favorites DB
		if logrus.GetLevel() < logrus.DebugLevel {
			_ = os.Setenv("DB_LOG_SILENT", "true")
		}
		app.db, err = favorites.NewDBService(cfg.FavoritesDBURL)
		if err != nil {
			logrus.WithError(err).Fatal("unable to connect to favorites database")
		}
		logrus.WithField("dsn", cfg.FavoritesDBURL).Debug("Connected to favorites database")
	} else {
		logrus.Warning("no favorites database configured")
	}

	app.trSvc, err = translate.New(app.db, cfg.LibreTranslateURL)
	if err != nil {
		logrus.WithError(err).Fatal("unable to init translation service")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)

	logrus.WithField("addr", cfg.ListenAddr).Info("starting server")
	errChan := make(chan error, 1)
	svr, err := api.NewServer(ctx, cfg, app.db, app.trSvc)
	if err != nil {
		logrus.WithError(err).Fatal("unable to init server")
	}
	svr.StartServer(errChan)

	select {
	case signo := <-sigc:
		logrus.WithField("signal", signo).Info("got signal")
	case <-ctx.Done():
		logrus.Info("got context done")
	case err := <-errChan:
		if !errors.Is(err, http.ErrServerClosed) {
			logrus.WithError(err).Error("API server finished normally")
		}
	}
	_ = svr.Stop(ctx)

	logrus.Info("server exiting")

}
