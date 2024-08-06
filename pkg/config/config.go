package config

import (
	"fmt"
	"github.com/dsggregory/config"
	"github.com/sirupsen/logrus"
	"os"
	"strings"
)

const (
	// DefaultLibreTranslateURL the default LibreTranslate API server URL
	DefaultLibreTranslateURL = "http://localhost:6001"
	// DefaultFavoritesDBURL the default is relative to the current working directory
	DefaultFavoritesDBURL = "pwd"
)

// CommonConfig settings common to any user of this pkg
type CommonConfig struct {
	// Debug panic, fatal, error, warn, info, debug, trace
	Debug             string `usage:"Turn on debug mode"`
	LibreTranslateURL string `usage:"Libre Translate URL"`
	FavoritesDBURL    string `usage:"Favorites DB URL"`
}

// APIConfig settings for the API web server only
type APIConfig struct {
	// ListenAddr address to listen for client connections to this server
	ListenAddr string `usage:"address to listen for client connections to this server"`

	StaticPages string `usage:"path to static web pages relative to where the server is started"`
}

// AppSettings settings for the application
type AppSettings struct {
	CommonConfig `flag:""`
	APIConfig    `flag:""`
}

// ReadConfig using default values from the arg, read config settings from cmdline or environment. Modifies the pointer to defaults.
func ReadConfig(defaults *AppSettings) error {
	if defaults.FavoritesDBURL == DefaultFavoritesDBURL {
		pwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("%w; cannot get current working directory", err)
		}
		defaults.FavoritesDBURL = "file://" + pwd + "/favorites.db"
	}

	err := config.ReadConfig(defaults)
	if err != nil {
		return fmt.Errorf("configuration loading failed")
	}

	if defaults.Debug != "" {
		lvl, err := logrus.ParseLevel(strings.ToLower(defaults.Debug))
		if err != nil {
			logrus.SetLevel(logrus.DebugLevel)
			logrus.WithField("lvl", defaults.Debug).Warn("unknown log level defaulting to DEBUG")
		} else {
			logrus.SetLevel(lvl)
		}
	}

	return nil
}
