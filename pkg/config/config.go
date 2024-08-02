package config

import (
	"fmt"
	"github.com/dsggregory/config"
	"github.com/sirupsen/logrus"
	"strings"
)

type AppSettings struct {
	Debug             string
	LibreTranslateURL string `usage:"Libre Translate URL"`
	FavoritesDBURL    string `usage:"Favorites DB URL"`
}

// ReadConfig using default values from the arg, read config settings from cmdline or environment. Modifies the pointer to defaults.
func ReadConfig(defaults *AppSettings) error {
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
