package api

import (
	"context"
	"encoding/json"
	"esptrans/pkg/config"
	"esptrans/pkg/favorites"
	"esptrans/pkg/translate"
	"fmt"
	"github.com/gorilla/mux"
	"html/template"
	"net/http"
	"strconv"
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
	trSvc         *translate.Translate
}

type MidWareFunc func(next http.Handler) http.Handler

func (s *Server) SetSvr(svr *http.Server) {
	s.svr = svr
}

// index renders the dashboard index page, displaying the created credential
// as well as any other credentials previously registered by the authenticated
// user.
func (s *Server) index(w http.ResponseWriter, _ *http.Request) {
	res := translate.Response{}
	res.DetectedLanguage.Language = translate.English
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

	lang, ok := values["srclang"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	srcLang = lang[0]
	if srcLang == translate.English {
		targetLang = translate.Spanish
	} else {
		targetLang = translate.English
	}
	vtxt, ok := values["input"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	trtext = vtxt[0]

	var skipFav bool
	skipFavStr, ok := values["skipFav"]
	if ok && len(skipFavStr) > 0 {
		skipFav = true
	}

	opts := translate.TranslateOptions{
		InLang:       srcLang,
		OutLang:      targetLang,
		SkipFavorite: skipFav,
	}
	trresp, err := s.trSvc.Translate(&opts, trtext)
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

func (s *Server) flashcards(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	values, accept := GetRequestParams(r)
	_ = values

	limit := 5
	if ls, ok := values["limit"]; ok {
		l, err := strconv.Atoi(ls[0])
		if err != nil || l <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		limit = l
	}
	favs, err := s.db.SelectRandomFavorites(limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// respond
	accept = NegotiateContentType(r, []string{CtAny, CtJson, CtHtml}, accept)
	switch accept {
	case CtJson:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(favs)
	case CtHtml, CtAny:
		_ = s.renderTemplate(w, "flashcards.gohtml", favs)
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
	s.mux.Handle("/flashcards", l(http.HandlerFunc(s.flashcards))).Methods(http.MethodGet)

	// for static pages e.g. javascript
	s.mux.PathPrefix("/").Handler(l(http.FileServer(http.Dir(s.cfg.StaticPages))))

	return nil
}

// RdbSessionStore redis DB number to be used. Refer to redis.RedisDBNo in sdk/redis.
const RdbSessionStore = 10

// NewServer creates an instance of the API. See StartServer().
func NewServer(ctx context.Context, cfg *config.AppSettings, mdb *favorites.DBService, trSvc *translate.Translate) (*Server, error) {
	s := &Server{
		cfg:           cfg,
		db:            mdb,
		trSvc:         trSvc,
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
