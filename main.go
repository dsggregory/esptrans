package main

import (
	"encoding/json"
	"esptrans/pkg/config"
	"esptrans/pkg/favorites"
	"esptrans/pkg/libre_translate"
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"
	"io"
	"os"
	"strings"
)

// App describes options used to translate
type App struct {
	cfg     *config.AppSettings
	inLang  string
	outLang string
	verbose bool
	db      *favorites.DBService
	lt      *libre_translate.LTClient
}

func canonicalizeString(s string) string {
	// lowercase if not a phrase (w/out punctuation)
	if !strings.ContainsAny(s, "?.!") {
		s = strings.ToLower(s)
	}
	// Fixup input text - removes sp, nl, quotes
	s = strings.Trim(s, " \"\r\n")

	return s
}

func doTranslate(app *App, sdata string) {
	sdata = canonicalizeString(sdata)

	res, err := app.lt.Translate(sdata, app.inLang, app.outLang)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to translate")
		return
	}
	if app.verbose {
		jd, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			logrus.WithError(err).Fatal("Failed to marshal json")
		}
		fmt.Println(string(jd))
	} else {
		fmt.Println(res.TranslatedText)
	}

	if app.db != nil {
		alts := []string{res.TranslatedText}
		alts = append(alts, res.Alternatives...)
		fav := favorites.Favorite{
			Source:     sdata,
			Target:     alts,
			SourceLang: app.inLang,
			TargetLang: app.outLang,
		}
		if res.DetectedLanguage.Language != "" {
			fav.SourceLang = res.DetectedLanguage.Language
		}
		_, err = app.db.AddFavorite(&fav)
		if err != nil {
			if !strings.Contains(err.Error(), "UNIQUE") {
				logrus.WithError(err).Fatal("Failed to add favorite")
			}
		}
	}
}

func main() {
	app := &App{}
	pwd, err := os.Getwd()
	if err != nil {
		logrus.WithError(err).Fatal("cannot get current working directory")
	}
	// config with defaults
	cfg := &config.AppSettings{
		Debug:             "INFO",
		LibreTranslateURL: "http://localhost:6001",
		FavoritesDBURL:    "file://" + pwd + "/favorites.db",
	}

	var inL, outL string = libre_translate.English, libre_translate.Spanish
	flag.StringVar(&inL, "i", "es", "Input language specification")
	o_lang := flag.Bool("r", false, "Translate es=>en. Default is inverse.")
	o_verbose := flag.Bool("v", false, "Verbose output")

	if err := config.ReadConfig(cfg); err != nil {
		logrus.Fatal(err)
	}
	if o_lang != nil && *o_lang {
		inL = libre_translate.Spanish
		outL = libre_translate.English
	} else {
		if inL != libre_translate.English && inL != libre_translate.Any {
			inL = libre_translate.English
			outL = libre_translate.Spanish
		}
	}
	logrus.WithFields(logrus.Fields{"inL": inL, "outL": outL}).Debug("Starting")
	app.cfg = cfg
	app.inLang = inL
	app.outLang = outL
	app.verbose = o_verbose != nil && *o_verbose == true

	// open the favorites DB
	if logrus.GetLevel() < logrus.DebugLevel {
		_ = os.Setenv("DB_LOG_SILENT", "true")
	}
	app.db, err = favorites.NewDBService(cfg.FavoritesDBURL)
	if err != nil {
		logrus.WithError(err).Fatal("unable to connect to favorites database")
	}
	logrus.WithField("dsn", cfg.FavoritesDBURL).Debug("Connected to favorites database")

	app.lt = libre_translate.New(cfg.LibreTranslateURL)

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

	doTranslate(app, sdata)
}
