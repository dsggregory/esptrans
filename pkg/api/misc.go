package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	AuthTypeNone = iota
	AuthTypeAPIKey
	AuthTypeBearer
)

const APIKeyHeader = "X-API-KEY"

// Authorization a parsed HTTP "Authorization" request header
type Authorization struct {
	// Type AuthTypeNone, et.al.
	Type int
	// Value the authorization data
	Value string
}

// GetRequestAuthorization parse the Authorization header from the HTTP request. Returns nil if not there.
func GetRequestAuthorization(req *http.Request) *Authorization {
	auth := &Authorization{}

	h := req.Header.Get(APIKeyHeader)
	if h != "" {
		auth.Type = AuthTypeAPIKey
		auth.Value = h
	} else {
		v := req.Header.Get("Authorization")
		if v != "" {
			i := strings.Index(v, " ")
			if i < 0 || strings.ToLower(v[0:i]) != "bearer" {
				return nil
			}
			auth.Type = AuthTypeBearer
			for ; i < len(v); i++ {
				if v[i] != ' ' {
					break
				}
			}
			auth.Value = v[i:]
		} else {
			for _, cv := range req.Cookies() {
				if cv.Name == "access_token" {
					auth.Type = AuthTypeBearer
					auth.Value = cv.Value
				}
			}
		}
	}

	if auth.Type == AuthTypeNone {
		return nil
	}

	return auth
}

// ErrResponse the form of an internal error returned to client
type ErrResponse struct {
	Msg string `json:"error"`
}

// RespondError respond to caller on internal errors in JSON.
// See http.Error() for a plaintext version.
func RespondError(w http.ResponseWriter, status int, err error) {
	e := ErrResponse{Msg: err.Error()}
	jmsg, _ := json.Marshal(&e)

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(jmsg)
}

// GetRequestParams returns either application/x-www-form-urlencoded params or those from Query()
func GetRequestParams(r *http.Request) (url.Values, string) {
	var values url.Values
	defaultAccept := CtHtml

	if k := r.Header.Get("Content-Type"); k == CtFormEnc {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		values, _ = url.ParseQuery(string(body))
	} else {
		defaultAccept = CtJson
		values = r.URL.Query()
	}

	return values, defaultAccept
}

func rvUint(r *http.Request, varName string) (uint, error) {
	vars := mux.Vars(r)
	idStr, ok := vars[varName]
	if !ok {
		vals, _ := GetRequestParams(r)
		idStr = vals.Get(varName)
		if idStr == "" {
			return uint(0), fmt.Errorf("request var %s not found", varName)
		}
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, fmt.Errorf("request var %s not an int", varName)
	}

	return uint(id), nil
}
func GetRequestVarInt(r *http.Request, varName string) (int, error) {
	x, err := rvUint(r, varName)
	if err == nil {
		return int(x), nil
	}
	return 0, err
}
func GetRequestVarUint(r *http.Request, varName string) (uint, error) {
	return rvUint(r, varName)
}
