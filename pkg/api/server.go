package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"esptrans/pkg/config"
	"esptrans/pkg/favorites"
	"esptrans/pkg/translate"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"html/template"
	"math/rand"
	"net/http"
	"net/url"
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

// TranslateResponse a composite struct containing all needed in translation
type TranslateResponse struct {
	*translate.TranslateOptions
	*translate.Response
}

func (s *Server) translate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		trresp := &TranslateResponse{
			TranslateOptions: &translate.TranslateOptions{},
			Response:         &translate.Response{},
		}
		trresp.DetectedLanguage.Language = translate.English
		_ = s.renderTemplate(w, "translationForm.gohtml", trresp)
		return
	}

	// expect application/x-www-form-urlencoded
	values, accept := GetRequestParams(r)
	srcLang, targetLang := translate.English, translate.Spanish
	var trtext string

	vtxt, ok := values["input"]
	if !ok {
		logrus.WithField("state", "form").Error("input is required")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	trtext = vtxt[0]

	opts := translate.TranslateOptions{
		InLang:  srcLang,
		OutLang: targetLang,
	}
	resp, err := s.trSvc.Detect(&opts, trtext)
	if err != nil {
		logrus.WithError(err).Error("argos failed")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	trresp := &TranslateResponse{TranslateOptions: &opts, Response: resp}
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

func (s *Server) newFavorite(res *TranslateResponse) *favorites.Favorite {
	alts := translate.CanonicalizeTranslations(res.Response)
	fav := favorites.Favorite{
		Source:     res.Input,
		Target:     alts,
		SourceLang: res.InLang,
		TargetLang: res.OutLang,
	}
	if res.DetectedLanguage.Language != "" {
		fav.SourceLang = res.DetectedLanguage.Language
	}

	return &fav
}

// saveTranslationAsFavorite save the translation as a favorite
func (s *Server) saveTranslationAsFavorite(res *TranslateResponse) error {
	if s.db != nil {
		_, err := s.db.AddFavorite(s.newFavorite(res))
		if err != nil {
			if !strings.Contains(err.Error(), "UNIQUE") {
				return fmt.Errorf("error adding favorite: %w", err)
			}
		}
	}
	return nil
}

// FlashcardResponse data used in flashcard display form
type FlashcardResponse struct {
	QuizLanguage string `json:"quizLanguage"`
	favorites.Favorite
}

// flashcardResponse based on the quiz language, arrange the flashcard result
func (s *Server) flashcardResponse(fav favorites.Favorite, values url.Values) FlashcardResponse {
	var quizLanguage string
	ql, ok := values["quizLanguage"]
	if !ok {
		quizLanguage = translate.English
	} else {
		quizLanguage = ql[0]
	}

	if quizLanguage != fav.SourceLang {
		// use a random item from Target
		randTarget := 0
		if len(fav.Target) > 1 {
			randTarget = rand.Intn(len(fav.Target))
		}
		target := append([]string{fav.Source}, fav.Target...) // mixed languages - maybe ok?
		// reverse it
		rfav := favorites.Favorite{
			SourceLang: fav.TargetLang,
			TargetLang: fav.SourceLang,
			Source:     fav.Target[randTarget],
			Target:     target,
		}
		rfav.ID = fav.ID
		fav = rfav
	}

	fcResp := FlashcardResponse{
		QuizLanguage: quizLanguage,
		Favorite:     fav,
	}

	return fcResp
}

// flashcards display a specific or random favorite
func (s *Server) flashcards(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		logrus.Error("Database not defined for flashcards")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	values, accept := GetRequestParams(r)

	var fav *favorites.Favorite
	var err error
	ids, ok := values["id"]
	if ok {
		id, err := strconv.Atoi(ids[0])
		if err != nil {
			logrus.Error("id param malformed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		fav, err = s.db.SelectFavorite(uint(id))
	} else {
		fav, err = s.db.SelectRandomFavorite()
	}
	if err != nil {
		logrus.WithError(err).Error("select failed")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fcResp := s.flashcardResponse(*fav, values)

	// respond
	accept = NegotiateContentType(r, []string{CtAny, CtJson, CtHtml}, accept)
	switch accept {
	case CtJson:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fcResp)
	case CtHtml, CtAny:
		_ = s.renderTemplate(w, "flashcards.gohtml", fcResp)
	}
}

// favoriteSave POST to save the current translation as a favorite
func (s *Server) favoriteSaveNew(w http.ResponseWriter, r *http.Request) {
	vars, _ := GetRequestParams(r)
	data := vars.Get("form-data-json")
	var res TranslateResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		logrus.WithError(err).Error("unmarshal favorite failed")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	/**
	if err := s.saveTranslationAsFavorite(res.Input, &res); err != nil {
		logrus.WithError(err).Error("save favorite failed")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	*/

	_ = s.renderTemplate(w, "favoriteEditReq.gohtml", s.newFavorite(&res))
}

// favoriteEdit display the form to edit an existing favorite
//  1. edit favorite by searching for ID/Name - favoritesImportReq (e.g. manage favorites)
//     This should arguably be moved to support LIKE name with resulting list of matches to select before edit
//  2. edit favorite of a known ID - flashcards, favoriteEditReq
func (s *Server) favoriteEdit(w http.ResponseWriter, r *http.Request) {
	vars, _ := GetRequestParams(r)
	fid := vars.Get("fav") // id or 'source'

	var source string
	id, err := strconv.Atoi(fid)
	if err != nil {
		if len(fid) > 0 {
			source = fid
		} else {
			mv := mux.Vars(r)
			fid = mv["id"]
			id, err = strconv.Atoi(fid)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
	}

	var fav *favorites.Favorite
	if id > 0 {
		fav, err = s.db.SelectFavorite(uint(id))
	} else {
		fav, err = s.db.SelectFavoriteSource(source)
	}
	if err != nil {
		RespondError(w, http.StatusBadRequest, fmt.Errorf("%w; select failed", err))
		return
	}

	_ = s.renderTemplate(w, "favoriteEditReq.gohtml", fav)
}

// favoriteEditSave save the favorite after modifications
func (s *Server) favoriteEditSave(w http.ResponseWriter, r *http.Request) {
	id, err := GetRequestVarUint(r, "id")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	infav := &favorites.Favorite{}
	if id != 0 { // if not new
		infav, err = s.db.SelectFavorite(id)
		if err != nil {
			logrus.WithError(err).Error("select failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	values, _ := GetRequestParams(r)

	// srclang source targets
	srclang, ok := values["srclang"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	source, ok := values["source"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	targets, ok := values["targets"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	fav := favorites.Favorite{
		Model: gorm.Model{
			ID:        uint(id),
			CreatedAt: infav.CreatedAt,
			UpdatedAt: infav.UpdatedAt,
		},
		SourceLang: srclang[0],
		TargetLang: infav.TargetLang,
		Source:     source[0],
		Target:     strings.Split(targets[0], "\n"),
	}
	if len(fav.Target) == 0 {
		logrus.Error("favorite target is empty")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if len(fav.Target) > 1 && fav.Target[len(fav.Target)-1] == "" {
		fav.Target = fav.Target[0 : len(fav.Target)-1]
	}

	if fav.ID == 0 {
		_, err = s.db.AddFavorite(&fav)
	} else {
		err = s.db.UpdateFavorite(&fav)
	}
	if err != nil {
		RespondError(w, http.StatusInternalServerError, fmt.Errorf("%w; save/update failed", err))
		return
	}

	fcResp := s.flashcardResponse(fav, values)
	_ = s.renderTemplate(w, "flashcards.gohtml", fcResp)
}

// favorites display the form to edit or import favorites
func (s *Server) favorites(w http.ResponseWriter, r *http.Request) {
	_ = s.renderTemplate(w, "favoritesImportReq.gohtml", nil)
}

// favoritesDoImport translate a batch of words and save favorites
func (s *Server) favoritesDoImport(w http.ResponseWriter, r *http.Request) {
	values, _ := GetRequestParams(r)

	// TODO detect using first one and remove radio buttons from form
	data, ok := values["data"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	lang, ok := values["srclang"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	inlang := lang[0]
	outlang := translate.English
	if inlang == translate.English {
		outlang = translate.Spanish
	}
	opts := translate.TranslateOptions{
		InLang:  inlang,
		OutLang: outlang,
	}

	respErrors := []string{}
	// data is `lang` text to be translated per line of input - no multiline translated
	scanner := bufio.NewScanner(bytes.NewBufferString(data[0]))
	for scanner.Scan() {
		src := strings.Trim(scanner.Text(), " \t\r\n")
		resp, err := s.trSvc.Translate(&opts, src)
		if err != nil {
			respErrors = append(respErrors, err.Error())
		}
		trresp := TranslateResponse{TranslateOptions: &opts, Response: resp}
		if err := s.saveTranslationAsFavorite(&trresp); err != nil {
			respErrors = append(respErrors, err.Error())
		}
	}

	_ = s.renderTemplate(w, "favoritesImportResp.gohtml", respErrors)
}

// favoriteDelete delete a favorite
func (s *Server) favoriteDelete(w http.ResponseWriter, r *http.Request) {
	id, err := GetRequestVarUint(r, "id")
	if err != nil {
		logrus.Error("id required")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	err = s.db.DeleteFavorite(id)

	// respond with the next flashcard
	fav, _ := s.db.SelectRandomFavorite()
	fcResp := s.flashcardResponse(*fav, nil)
	// respond
	accept := NegotiateContentType(r, []string{CtAny, CtJson, CtHtml}, CtHtml)
	switch accept {
	case CtJson:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fcResp)
	case CtHtml, CtAny:
		_ = s.renderTemplate(w, "flashcards.gohtml", fcResp)
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

	s.mux.Handle("/translate", l(http.HandlerFunc(s.translate))).Methods(http.MethodGet, http.MethodPost)
	s.mux.Handle("/flashcards", l(http.HandlerFunc(s.flashcards))).Methods(http.MethodGet)
	s.mux.Handle("/flashcard/{id}", l(http.HandlerFunc(s.flashcards))).Methods(http.MethodGet)
	s.mux.Handle("/favorites", l(http.HandlerFunc(s.favorites))).Methods(http.MethodGet)
	s.mux.Handle("/favorites", l(http.HandlerFunc(s.favoritesDoImport))).Methods(http.MethodPost)
	s.mux.Handle("/favorite", l(http.HandlerFunc(s.favoriteSaveNew))).Methods(http.MethodPost)
	s.mux.Handle("/favorite/edit", l(http.HandlerFunc(s.favoriteEdit))).Methods(http.MethodGet, http.MethodPost)
	s.mux.Handle("/favorite/edit/{id}", l(http.HandlerFunc(s.favoriteEdit))).Methods(http.MethodGet)
	s.mux.Handle("/favorite/{id}", l(http.HandlerFunc(s.favoriteEditSave))).Methods(http.MethodPut)
	s.mux.Handle("/favorite/{id}", l(http.HandlerFunc(s.favoriteDelete))).Methods(http.MethodDelete)

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

	rand.Seed(time.Now().UnixNano())

	return s, nil
}
