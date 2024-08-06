package translate

import (
	"errors"
	"esptrans/pkg/favorites"
	"fmt"
	"github.com/sirupsen/logrus"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// Translate an instance of this service
type Translate struct {
	DB *favorites.DBService
	LT *LTClient
	argosAPIProc
}

// argosAPIProc our argos API process
type argosAPIProc struct {
	cmd  *exec.Cmd
	done chan error
}

type TranslateOptions struct {
	InLang       string
	OutLang      string
	SkipFavorite bool
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

func (t *Translate) saveFavorite(opts *TranslateOptions, source string, res *Response) error {
	if t.DB != nil {
		alts := CanonicalizeTranslations(res)
		fav := favorites.Favorite{
			Source:     source,
			Target:     alts,
			SourceLang: opts.InLang,
			TargetLang: opts.OutLang,
		}
		if res.DetectedLanguage.Language != "" {
			fav.SourceLang = res.DetectedLanguage.Language
		}
		_, err := t.DB.AddFavorite(&fav)
		if err != nil {
			if !strings.Contains(err.Error(), "UNIQUE") {
				return fmt.Errorf("error adding favorite: %w", err)
			}
		}
	}
	return nil
}

func CanonicalizeTranslations(res *Response) []string {
	// use a map to avoid dups and maintain order in resulting array
	malts := make(map[string]bool)
	malts[res.TranslatedText] = true
	alts := []string{res.TranslatedText}
	for _, x := range res.Alternatives {
		if _, ok := malts[x]; !ok {
			alts = append(alts, x)
		}
		malts[x] = true
	}

	return alts
}

// Translate calls the LibreTranslate wrapper and saves to favorites
func (t *Translate) Translate(opts *TranslateOptions, sdata string) (*Response, error) {
	if len(sdata) == 0 {
		return nil, fmt.Errorf("empty string")
	}
	sdata = canonicalizeString(sdata)

	res, err := t.LT.Translate(sdata, opts.InLang, opts.OutLang)
	if err != nil {
		return nil, fmt.Errorf("Failed to translate: %w", err)
	}

	if !opts.SkipFavorite {
		if err = t.saveFavorite(opts, sdata, res); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// manageArgos start our argos API server in a go routine
func (t *Translate) manageArgos() error {
	t.done = make(chan error)

	cmdPath, err := exec.LookPath("python3")
	if err != nil {
		return fmt.Errorf("%w; python3", err)
	}
	script := "./argostranslate-api.py"
	if _, err := os.Stat(script); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w; %s", err, script)
	}

	u, err := url.Parse(t.LT.LibreTranslateURL)
	if err != nil {
		return err
	}
	t.cmd = exec.Command(cmdPath, script, "--listen", u.Host)
	t.cmd.Stderr = os.Stderr
	t.cmd.Stdout = os.Stdout
	go func() {
		err := t.cmd.Run()
		if t.cmd.ProcessState != nil {
			_ = t.cmd.Wait()
			if status := t.cmd.ProcessState.ExitCode(); status != 0 {
				err = fmt.Errorf("%w; %d", err, status)
			}
		}
		t.done <- err
		close(t.done)
	}()

	logrus.WithField("addr", u.Host).Info("started our argos API server")
	return nil
}

// Done waits on our argos API command to exit
func (t *Translate) Done() error {
	if t.done == nil {
		return nil
	}
	return <-t.done
}

// Close parent can call this to close resources
func (t *Translate) Close() error {
	if t.done != nil {
		_ = t.cmd.Process.Kill() // the goroutine in manage() will send the error
		_ = t.cmd.Wait()
		logrus.Info("argos API server exiting")
		return t.Done()
	}
	return nil
}

// New creates a new instance of the translation API wrapper
func New(db *favorites.DBService, apiURL string) (*Translate, error) {
	t := &Translate{
		DB: db,
		LT: NewLibreTranslate(apiURL),
	}

	if err := t.manageArgos(); err != nil {
		return nil, err
	}

	return t, nil
}
