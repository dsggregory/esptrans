package api

import (
	"context"
	"encoding/json"
	"esptrans/pkg/config"
	"esptrans/pkg/favorites"
	"esptrans/pkg/libre_translate"
	"fmt"
	"github.com/gorilla/mux"
	"html/template"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Server struct {
	cfg *config.AppSettings
	// mux we use gorilla mux so we can handle query path parsing
	mux           *mux.Router
	svr           *http.Server
	templates     *template.Template
	wg            *sync.WaitGroup
	logMiddleware MidWareFunc
	db            *favorites.DBService
	lt            *libre_translate.LTClient
}

type MidWareFunc func(next http.Handler) http.Handler

func (s *Server) SetSvr(svr *http.Server) {
	s.svr = svr
}

// index renders the dashboard index page, displaying the created credential
// as well as any other credentials previously registered by the authenticated
// user.
func (s *Server) index(w http.ResponseWriter, _ *http.Request) {
	res := libre_translate.Response{}
	res.DetectedLanguage.Language = libre_translate.English
	_ = s.renderTemplate(w, "dashboard.gohtml", &res)
}

func (s *Server) template(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tmpl, ok := vars["name"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// an HTMX hx-trigger name for content replacement (if you are inclined to use)
	w.Header().Set("HX-Trigger", tmpl)

	var v any
	_ = json.NewDecoder(r.Body).Decode(&v)
	_ = r.Body.Close()

	_ = s.renderTemplate(w, tmpl, nil)
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) addHealthRoutes(l MidWareFunc) {
	s.mux.Handle("/health", l(http.HandlerFunc(health)))
	s.mux.Handle("/health/{any}", l(http.HandlerFunc(health))) // I like to configure /health/liveness as liveness probe endpoint
}

func (s *Server) translate(w http.ResponseWriter, r *http.Request) {
	// expect application/x-www-form-urlencoded
	values, accept := GetRequestParams(r)

	var srcLang, targetLang, trtext string

	lang, ok := values["inputLang"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	srcLang = lang[0]
	var vtxt []string
	if srcLang == libre_translate.English {
		targetLang = libre_translate.Spanish
		vtxt, ok = values["enInp"]
	} else {
		targetLang = libre_translate.English
		vtxt, ok = values["esInp"]
	}
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	trtext = vtxt[0]

	trresp, err := s.lt.Translate(trtext, srcLang, targetLang)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if trresp.DetectedLanguage.Language == "" {
		trresp.DetectedLanguage.Language = srcLang // so it's available in the form
	}

	// respond
	accept = NegotiateContentType(r, []string{CtAny, CtJson, CtHtml}, accept)
	switch accept {
	case CtJson:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(trresp)
	case CtHtml, CtAny:
		_ = s.renderTemplate(w, "translationForm.gohtml", trresp)
	}
}

// Stop shuts down the web server
func (s *Server) Stop(ctx context.Context) error {
	if s.svr == nil {
		return nil
	}
	err := s.svr.Shutdown(ctx)
	if err == nil {
		s.wg.Wait()
	}
	return err
}

// StartServer starts the proxy web service and writes to `errc` when the service exits. The returned server and waitgroup are to be used by the caller during shutdown.
func (s *Server) StartServer(errc chan<- error) {
	s.svr = &http.Server{
		Addr:         s.cfg.ListenAddr,
		Handler:      s.mux,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	s.wg = &sync.WaitGroup{}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		errc <- s.svr.ListenAndServe()
	}()
}

// LogRoutes dump routes to log for debug purposes
func (s *Server) LogRoutes() {
	rsp := "Current Routes:\n"
	_ = s.mux.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		tpl, _ := route.GetPathTemplate()
		met, _ := route.GetMethods()
		rsp += tpl + "\t\t" + strings.Join(met, ",") + "\n"
		return nil
	})
	fmt.Println(rsp)
}

func (s *Server) newRouter() error {
	var l func(next http.Handler) http.Handler = s.logMiddleware

	s.addHealthRoutes(l)
	//s.mux.Handle("/dashboard", l(s.adminLoginRequired(http.HandlerFunc(s.index), false))).Methods(http.MethodGet)
	s.mux.Handle("/", l(http.HandlerFunc(s.index))).Methods(http.MethodGet)
	s.mux.Handle("/template/{name}", l(http.HandlerFunc(s.template))).Methods(http.MethodGet, http.MethodPost)

	s.mux.Handle("/translate", l(http.HandlerFunc(s.translate))).Methods(http.MethodPost)

	// for static pages e.g. javascript
	s.mux.PathPrefix("/").Handler(l(http.FileServer(http.Dir(s.cfg.StaticPages))))

	return nil
}

// RdbSessionStore redis DB number to be used. Refer to redis.RedisDBNo in sdk/redis.
const RdbSessionStore = 10

// NewServer creates an instance of the API. See StartServer().
//
// db is the database service and cannot be nil.
//
// webauthnService is an already-configured webauthn RP.  If nil, one is created from the cfg.
func NewServer(ctx context.Context, cfg *config.AppSettings, mdb *favorites.DBService, lt *libre_translate.LTClient) (*Server, error) {
	s := &Server{
		cfg:           cfg,
		db:            mdb,
		lt:            lt,
		mux:           mux.NewRouter(),
		logMiddleware: NewLoggingMiddleware,
	}
	if err := s.LoadTemplates(); err != nil {
		return nil, err
	}

	// add routes
	if err := s.newRouter(); err != nil {
		return nil, err
	}

	//s.LogRoutes()

	return s, nil
}