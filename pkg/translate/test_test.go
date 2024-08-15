package translate

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

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

	Convey("Guess input language", t, func() {

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
			So(ltresp.TranslatedText, ShouldResemble, tc.expXlate)
			So(len(ltresp.Alternatives), ShouldBeGreaterThan, 0)
		}

		ltresp, err := trSvc.Translate(opts, "failed translate")
		So(err, ShouldNotBeNil)
		So(ltresp, ShouldBeNil)
	})
}
