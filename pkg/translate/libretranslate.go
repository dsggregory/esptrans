package translate

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const (
	LTURL = "http://localhost:6001/"
)

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

// Health check the health of the server
func (l *LTClient) Health() error {
	resp, err := http.Get(l.LibreTranslateURL + "/frontend/settings")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}
	return nil
}

// New creates an instance of this service
func NewLibreTranslate(url string) *LTClient {
	return &LTClient{LibreTranslateURL: url}
}
