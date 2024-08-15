package translate

import (
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Translate an instance of this service
type Translate struct {
	LT *LTClient
	argosAPIProc
}

// argosAPIProc our argos API process
type argosAPIProc struct {
	scriptPath string
	cmd        *exec.Cmd
	done       chan error
}

type TranslateOptions struct {
	InLang  string
	OutLang string
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

// Detect try to translate with both languages as source and take the results with the best confidence
func (t *Translate) Detect(opts *TranslateOptions, sdata string) (*Response, error) {
	t1, err := t.Translate(opts, sdata)
	if err != nil {
		return nil, err
	}
	o2 := TranslateOptions{InLang: opts.OutLang, OutLang: opts.InLang}
	t2, err := t.Translate(&o2, sdata)
	if err != nil {
		return nil, err
	}

	// Confidence of zero is poor. The best of two confidences is the one with the value closer (but not equal to) to zero.
	if t2.DetectedLanguage.Confidence > t1.DetectedLanguage.Confidence {
		return t2, nil
	} else {
		return t1, nil
	}
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

	return res, nil
}

// manageArgos start our argos API server in a go routine
func (t *Translate) manageArgos() error {
	t.done = make(chan error)

	cmdPath, err := exec.LookPath("python3")
	if err != nil {
		return fmt.Errorf("%w; python3", err)
	}
	script := t.scriptPath
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

	// wait on the service to be available
	var isUp bool
	for i := 0; i < 5; i++ {
		if err := t.LT.Health(); err != nil {
			time.Sleep(2 * time.Second)
		} else {
			isUp = true
			break
		}
	}
	if !isUp {
		logrus.Warn("timed-out waiting on Argos service to become available")
	}

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

// WithAPIURL a functional option to specify where to run the Argos API
func WithAPIURL(apiURL string) func(*Translate) {
	return func(t *Translate) {
		t.LT = NewLibreTranslate(apiURL)
	}
}

// WithArgosScript a functional option to specify the location of our Argos python script. The default is "./argostranslate-api.py".
func WithArgosScript(scriptPath string) func(*Translate) {
	return func(t *Translate) {
		t.scriptPath = scriptPath
	}
}

// WithoutArgos a functional option to indicate we will not manage the Argos/LibreTranslate API server. Can use this with testing when mocking that service.
func WithoutArgos() func(*Translate) {
	return func(t *Translate) {
		t.scriptPath = ""
	}
}

// New creates a new instance of the translation API wrapper
func New(options ...func(*Translate)) (*Translate, error) {
	t := &Translate{
		LT: nil,
		argosAPIProc: argosAPIProc{
			scriptPath: "./argostranslate-api.py",
		},
	}

	for _, option := range options {
		option(t)
	}
	if t.LT == nil {
		return nil, errors.New("WithAPIURL is a required option")
	}

	if t.scriptPath != "" {
		if err := t.manageArgos(); err != nil {
			return nil, err
		}
	} else {
		logrus.Warn("not managing argos API server, by config")
	}

	return t, nil
}
