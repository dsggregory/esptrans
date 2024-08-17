package translate

import (
	"encoding/json"
	"esptrans/pkg/favorites"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func otherLang(lang string) string {
	switch lang {
	case English:
		return Spanish
	case Spanish:
		return English
	default:
		return Any
	}
}

func TestTranslate(t *testing.T) {
	Convey("Test canonicalize", t, func() {
		type cases struct {
			orig         string
			expCanonical string
		}
		tcs := []cases{
			{"TOMaré", "tomaré"},
			{`"tomaré"`, "tomaré"},
			{`"  tomaré"
`, "tomaré"},
		}
		for _, tc := range tcs {
			s := canonicalizeString(tc.orig)
			So(s, ShouldEqual, tc.expCanonical)
		}
	})

	Convey("Test stringsResemble", t, func() {
		type cases struct {
			a      string
			b      string
			expect bool
		}
		tcs := []cases{
			{"hello", "Hello.", true},
			{"hello?", "Hello", true},
			{"Hello_there", "hello there", false},
			{"hello", "Hola.", false},
		}
		for _, tc := range tcs {
			v := stringsResemble(tc.a, tc.b)
			So(v, ShouldEqual, tc.expect)
		}
	})

	Convey("Test mock argos", t, func() {
		// make a temp copy of the fixtures DB file
		fp, err := os.CreateTemp("", "*.flashcards")
		So(err, ShouldBeNil)
		defer func() {
			_ = os.Remove(fp.Name())
		}()
		fpo, err := os.Open("../../testdata/fixtures/favorites.db")
		So(err, ShouldBeNil)
		n, err := io.Copy(fp, fpo)
		_ = fpo.Close()
		_ = fp.Close()
		So(n, ShouldBeGreaterThan, 0)
		So(err, ShouldBeNil)

		db, err := favorites.NewDBService("file://" + fp.Name())
		So(err, ShouldBeNil)
		// db.Db().Logger = logger.Default

		// fake Argos/LibreTranslate server
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lreq := Request{}
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
			if err := db.Db().Where("source = ?", lreq.Q).First(fav).Error; err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			resp := Response{
				Input:          lreq.Source,
				Alternatives:   fav.Target,
				TranslatedText: fav.Target[0],
			}
			resp.DetectedLanguage.Language = fav.SourceLang
			respBody, err := json.Marshal(&resp)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(respBody)
		}))
		defer ts.Close()

		trsvc, err := New(
			WithoutArgos(),
			WithAPIURL(ts.URL),
		)
		So(err, ShouldBeNil)

		type tcase struct {
			input     string
			inputLang string
			expected  string
		}
		tcs := []tcase{
			{"hola", Spanish, "HELLO"},
			{"hello", English, "hola"},
			{"varón", Spanish, "male"},
			{"I'm going to bed.", English, "Me voy a la cama."},
			{"My preference is that I have no preference. But sometimes I prefer this to that.",
				English,
				"Mi preferencia es que no tengo preferencia. Pero a veces prefiero esto a eso.",
			},
		}
		opts := TranslateOptions{InLang: English, OutLang: Spanish}
		for _, tc := range tcs {
			t1, t2, err := trsvc.tryDetect(&opts, tc.input)
			So(err, ShouldBeNil)
			dt := trsvc.chooseDetection(t1, t2)
			So(dt.DetectedLanguage.Language, ShouldEqual, tc.inputLang)
			ec := stringsResemble(dt.TranslatedText, tc.expected)
			So(ec, ShouldBeTrue)

			resp, err := trsvc.Detect(&opts, tc.input)
			So(err, ShouldBeNil)
			So(resp.DetectedLanguage.Language, ShouldEqual, tc.inputLang)
			ec = stringsResemble(resp.TranslatedText, tc.expected)
			So(ec, ShouldBeTrue)
		}
	})

	SkipConvey("Test with real argos data", t, func() {
		Convey("Guess input language", func() {
			argosPort := "5678"
			argosURL := "http://localhost:" + argosPort
			trsvc, err := New(
				WithAPIURL(argosURL),
				WithArgosScript("../../argostranslate-api.py"),
			)
			So(err, ShouldBeNil)
			defer trsvc.Close()

			type tcase struct {
				input     string
				inputLang string
				expected  string
			}
			tcs := []tcase{
				{"hello", English, "hola"},
				{"hello", English, "hola"},
				{"hola.", Spanish, "HELLO"},
				{"My preference is that I have no preference. But sometimes I prefer this to that.",
					English,
					"Mi preferencia es que no tengo preferencia. Pero a veces prefiero esto a eso.",
				},
			}
			opts := TranslateOptions{InLang: English, OutLang: Spanish}
			for _, tc := range tcs {
				t1, t2, err := trsvc.tryDetect(&opts, tc.input)
				So(err, ShouldBeNil)
				dt := trsvc.chooseDetection(t1, t2)
				So(dt.DetectedLanguage.Language, ShouldEqual, tc.inputLang)
				ec := stringsResemble(dt.TranslatedText, tc.expected)
				So(ec, ShouldBeTrue)

				resp, err := trsvc.Detect(&opts, tc.input)
				So(err, ShouldBeNil)
				So(resp.DetectedLanguage.Language, ShouldEqual, tc.inputLang)
				ec = stringsResemble(resp.TranslatedText, tc.expected)
				So(ec, ShouldBeTrue)
			}
		})
	})

	Convey("Test translate", t, func() {
		type cases struct {
			orig     string
			expXlate string
		}
		tcs := []cases{
			{"The connection is bad", "la conexión es mala"},
			{"Hello", "hola"},
		}
		// fake LibreTranslate server
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lreq := Request{}
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
			transTxt := ""
			for _, tc := range tcs {
				if canonicalizeString(tc.orig) == canonicalizeString(lreq.Q) {
					transTxt = tc.expXlate
					break
				}
			}
			if transTxt == "" {
				w.WriteHeader(http.StatusNotFound)
			}
			resp := Response{
				Input:          lreq.Source,
				Alternatives:   []string{"none"},
				TranslatedText: transTxt,
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

		trSvc, err := New(
			WithAPIURL(ts.URL),
			WithoutArgos(), // not needed as it's mocked above
		)
		So(err, ShouldBeNil)

		opts := &TranslateOptions{
			InLang:  "en",
			OutLang: "es",
		}
		for _, tc := range tcs {
			ltresp, err := trSvc.Translate(opts, tc.orig)
			So(err, ShouldBeNil)
			ec := stringsResemble(ltresp.TranslatedText, tc.expXlate)
			So(ec, ShouldBeTrue)
		}

		ltresp, err := trSvc.Translate(opts, "failed translate")
		So(err, ShouldNotBeNil)
		So(ltresp, ShouldBeNil)
	})
}
