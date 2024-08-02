package translate

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTranslate(t *testing.T) {
	Convey("Test canonicalize", t, func() {
		type cases struct {
			orig         string
			expXlate     string
			expCanonical string
		}
		tcs := []cases{
			{"TOMaré", "i'll take it", "tomaré"},
			{`"tomaré"`, "i'll take it", "tomaré"},
			{`"  tomaré"
`, "i'll take it", "tomaré"},
		}
		for _, tc := range tcs {
			s := canonicalizeString(tc.orig)
			So(s, ShouldEqual, tc.expCanonical)
		}
	})
	Convey("Test translate", t, func() {

	})
}
