package main

import (
	"encoding/json"
	"esptrans/pkg/config"
	"esptrans/pkg/favorites"
	"esptrans/pkg/libre_translate"
	"esptrans/pkg/translate"
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"
	"io"
	"os"
)

// App describes options used to translate
type App struct {
	cfg     *config.AppSettings
	inLang  string
	outLang string
	verbose bool
	db      *favorites.DBService
	trSvc   *translate.Translate
}

func main() {
	app := &App{}
	// config with defaults
	cfg := &config.AppSettings{
		CommonConfig: config.CommonConfig{
			Debug:             "INFO",
			LibreTranslateURL: config.DefaultLibreTranslateURL,
			FavoritesDBURL:    config.DefaultFavoritesDBURL,
		},
	}

	var inL, outL string = libre_translate.English, libre_translate.Spanish
	o_lang := flag.Bool("r", false, "Translate es=>en. Default is inverse.")
	o_verbose := flag.Bool("v", false, "Verbose output")
	o_nosave := flag.Bool("n", false, "Do not save to favorites")

	err := config.ReadConfig(cfg)
	if err != nil {
		logrus.Fatal(err)
	}
	if o_lang != nil && *o_lang {
		inL = libre_translate.Spanish
		outL = libre_translate.English
	}
	logrus.WithFields(logrus.Fields{"inL": inL, "outL": outL}).Debug("Starting")

	app.cfg = cfg
	app.inLang = inL
	app.outLang = outL
	app.verbose = o_verbose != nil && *o_verbose == true

	// open the favorites DB
	if !(o_nosave != nil && *o_nosave == true) {
		if logrus.GetLevel() < logrus.DebugLevel {
			_ = os.Setenv("DB_LOG_SILENT", "true")
		}
		app.db, err = favorites.NewDBService(cfg.FavoritesDBURL)
		if err != nil {
			logrus.WithError(err).Fatal("unable to connect to favorites database")
		}
		logrus.WithField("dsn", cfg.FavoritesDBURL).Debug("Connected to favorites database")
	}

	app.trSvc, err = translate.New(app.db, cfg.LibreTranslateURL)
	if err != nil {
		logrus.WithError(err).Fatal("unable to init the translation service")
	}

	// input from cmdline or stdin
	var data []byte
	if len(flag.Args()) > 0 {
		data = []byte(flag.Arg(0))
	} else {
		// stdin
		if term.IsTerminal(int(os.Stdout.Fd())) {
			_, _ = fmt.Fprintf(os.Stderr, "Enter text to translate followed by ^D:\n")
		}
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			logrus.WithError(err).Fatal("Failed to read from stdin")
			return
		}
	}
	sdata := string(data)

	opts := &translate.TranslateOptions{
		InLang:  app.inLang,
		OutLang: app.outLang,
	}
	res, err := app.trSvc.Translate(opts, sdata)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to translate")
		return
	}
	if app.verbose {
		jd, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			logrus.WithError(err).Fatal("Failed to marshal JSON")
		}
		fmt.Println(string(jd))
	} else {
		fmt.Println(res.TranslatedText)
	}
}
