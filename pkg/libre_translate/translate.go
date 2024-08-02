package libre_translate

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const (
	LTURL = "http://localhost:6001/"

	English = "en"
	Spanish = "es"
	Any     = "auto"
)

type Request struct {
	Q            string `json:"q"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	Format       string `json:"format"`
	Alternatives int    `json:"alternatives"`
	APIKey       string `json:"api_key"`
}

type Response struct {
	Input            string   `json:"input,omitempty"`
	Alternatives     []string `json:"alternatives"`
	DetectedLanguage struct {
		Language   string  `json:"language"`
		Confidence float64 `json:"confidence"`
	} `json:"detectedLanguage"`
	TranslatedText string `json:"translatedText"`
}

// LTClient an instance of this service
type LTClient struct {
	LibreTranslateURL string
}

func (l *LTClient) translate(text string, source string, target string) (*Response, error) {
	reqdata := Request{
		Q:            text,
		Source:       source,
		Target:       target,
		Format:       "text",
		Alternatives: 3,
	}
	reqbody, err := json.Marshal(reqdata)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(l.LibreTranslateURL+"/translate", "application/json", bytes.NewBuffer(reqbody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}
	respbody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	res := Response{}
	if err = json.Unmarshal(respbody, &res); err != nil {
		return nil, err
	}
	res.Input = text

	return &res, nil
}

func (l *LTClient) EnToEs(text string) (*Response, error) {
	return l.translate(text, English, Spanish)
}

func (l *LTClient) EsToEn(text string) (*Response, error) {
	return l.translate(text, Spanish, English)
}

func (l *LTClient) Translate(text string, source string, target string) (*Response, error) {
	return l.translate(text, source, target)
}

func (l *LTClient) Auto(text string) (*Response, error) {
	return l.translate(text, Any, Any)
}

// New creates an instance of this service
func New(url string) *LTClient {
	return &LTClient{LibreTranslateURL: url}
}
