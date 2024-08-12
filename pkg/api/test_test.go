package api

import (
	"context"
	"encoding/json"
	"esptrans/pkg/config"
	"esptrans/pkg/favorites"
	"esptrans/pkg/translate"
	. "github.com/smartystreets/goconvey/convey"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

/****
Test GUI
Test flashcards
	Test quizLanguage swap
Test translate
*/

// remember to os.Remove(os.File.Name()) when you are finished
func newTestDatabase() (*favorites.DBService, *os.File, error) {
	// make a temp copy of the fixtures DB file
	fpwr, err := os.CreateTemp("", "*.flashcards")
	if err != nil {
		return nil, nil, err
	}
	fprd, err := os.Open("../../testdata/fixtures/favorites.db")
	if err != nil {
		_ = os.Remove(fpwr.Name())
		return nil, nil, err
	}
	_, err = io.Copy(fpwr, fprd)
	_ = fprd.Close()
	_ = fpwr.Close()
	if err != nil {
		_ = os.Remove(fpwr.Name())
		return nil, nil, err
	}

	db, err := favorites.NewDBService("file://" + fpwr.Name())

	return db, fpwr, err
}

func TestUI(t *testing.T) {
	Convey("Test flashcards", t, func() {
		Convey("check quiz lang", func() {
			s := Server{}
			fav := favorites.Favorite{
				SourceLang: translate.English,
				TargetLang: translate.Spanish,
				Source:     "Hello",
				Target:     []string{"Hola"},
			}
			values := url.Values{"quizLanguage": []string{translate.English}}
			r := s.flashcardResponse(fav, values)
			So(r.Source, ShouldEqual, fav.Source)
			So(r.SourceLang, ShouldEqual, fav.SourceLang)

			values = url.Values{"quizLanguage": []string{translate.Spanish}}
			r = s.flashcardResponse(fav, values)
			So(r.Source, ShouldEqual, fav.Target[0])
			So(r.SourceLang, ShouldEqual, fav.TargetLang)
		})
		Convey("try handler", func() {
			db, fp, err := newTestDatabase()
			So(err, ShouldBeNil)
			defer os.Remove(fp.Name())
			ctx := context.Background()
			cfg := config.AppSettings{
				CommonConfig: config.CommonConfig{
					Debug:             "",
					LibreTranslateURL: "",
					FavoritesDBURL:    "",
				},
				APIConfig: config.APIConfig{
					ListenAddr:  "",
					StaticPages: "../../views",
				},
			}
			s, err := NewServer(ctx, &cfg, db, nil)
			So(err, ShouldBeNil)
			// just test the handlers
			req, err := http.NewRequest("GET", "/flashcards?quizLanguage=en", http.NoBody)
			So(err, ShouldBeNil)

			// the response recorder (http.ResponseWriter)
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(s.flashcards)
			handler.ServeHTTP(rr, req)
			So(rr.Code, ShouldEqual, http.StatusOK)
			resp := FlashcardResponse{}
			err = json.Unmarshal(rr.Body.Bytes(), &resp)
			So(err, ShouldBeNil)
			So(resp.SourceLang, ShouldEqual, "en")
		})
	})
	Convey("Try endpoints", t, func() {
		db, fp, err := newTestDatabase()
		So(err, ShouldBeNil)
		defer os.Remove(fp.Name())
		ctx := context.Background()
		cfg := config.AppSettings{
			CommonConfig: config.CommonConfig{
				Debug:             "",
				LibreTranslateURL: "",
				FavoritesDBURL:    "",
			},
			APIConfig: config.APIConfig{
				ListenAddr:  "",
				StaticPages: "../../views",
			},
		}
		s, err := NewServer(ctx, &cfg, db, nil)
		So(err, ShouldBeNil)

		req, err := http.NewRequest("POST", "/edit?fav=1", http.NoBody)
		So(err, ShouldBeNil)
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(s.favoriteEdit)
		handler.ServeHTTP(rr, req)
		So(rr.Code, ShouldEqual, http.StatusOK)

		data := url.Values{"fav": []string{"2"}}
		req, err = http.NewRequest("POST", "/edit", strings.NewReader(data.Encode()))
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		So(err, ShouldBeNil)
		rr = httptest.NewRecorder()
		handler = http.HandlerFunc(s.favoriteEdit)
		handler.ServeHTTP(rr, req)
		So(rr.Code, ShouldEqual, http.StatusOK)

		data = url.Values{"fav": []string{"tomaré"}}
		req, err = http.NewRequest("POST", "/edit", strings.NewReader(data.Encode()))
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		So(err, ShouldBeNil)
		rr = httptest.NewRecorder()
		handler = http.HandlerFunc(s.favoriteEdit)
		handler.ServeHTTP(rr, req)
		So(rr.Code, ShouldEqual, http.StatusOK)
	})
	SkipConvey("Test UI translate", t, func() {
		// make a temp copy of the fixtures DB file
		fp, err := os.CreateTemp("", "*.flashcards")
		So(err, ShouldBeNil)
		defer func() {
			_ = os.Remove(fp.Name())
		}()
		fpo, err := os.Open("../../testdata/fixtures/favorites.db")
		So(err, ShouldBeNil)
		_, err = io.Copy(fpo, fp)
		_ = fpo.Close()
		_ = fp.Close()
		So(err, ShouldBeNil)

		db, err := favorites.NewDBService("file://" + fp.Name())
		So(err, ShouldBeNil)

		// fake Argos/LibreTranslate server
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lreq := translate.Request{}
			body, err := io.ReadAll(r.Body)
			_ = r.Body.Close()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			err = json.Unmarshal(body, &lreq)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			fav := &favorites.Favorite{Source: lreq.Q}
			if err := db.Db().Find(fav).Where(fav).First(fav).Error; err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			resp := translate.Response{
				Input:          lreq.Source,
				Alternatives:   fav.Target,
				TranslatedText: fav.Target[0],
			}
			respBody, err := json.Marshal(&resp)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(respBody)
		}))
		defer ts.Close()

		tsvc, err := translate.New(
			translate.WithoutArgos(),
			translate.WithAPIURL(ts.URL),
			translate.WithDB(db),
		)
		So(err, ShouldBeNil)

		ctx := context.Background()
		svr, err := NewServer(
			ctx,
			&config.AppSettings{},
			db,
			tsvc)
		So(err, ShouldBeNil)
		defer svr.Stop(ctx)

		resp, err := tsvc.Translate(&translate.TranslateOptions{
			InLang:       translate.English,
			OutLang:      translate.Spanish,
			SkipFavorite: true,
		},
			"Hi")
		So(err, ShouldBeNil)
		So(resp, ShouldNotBeNil)
	})
}
