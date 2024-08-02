package translate

import (
	"encoding/json"
	"esptrans/pkg/libre_translate"
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
			lreq := libre_translate.Request{}
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
			resp := libre_translate.Response{
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

		opts := &TranslateOptions{
			InLang:  "en",
			OutLang: "es",
			DB:      nil,
			LT:      libre_translate.New(ts.URL),
		}
		for _, tc := range tcs {
			ltresp, err := Translate(opts, tc.orig)
			So(err, ShouldBeNil)
			So(ltresp.TranslatedText, ShouldResemble, tc.expXlate)
			So(len(ltresp.Alternatives), ShouldBeGreaterThan, 0)
		}

		ltresp, err := Translate(opts, "failed translate")
		So(err, ShouldNotBeNil)
		So(ltresp, ShouldBeNil)
	})
}
