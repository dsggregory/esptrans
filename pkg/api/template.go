package api

import (
	"encoding/json"
	"esptrans/pkg/favorites"
	"esptrans/pkg/translate"
	"github.com/sirupsen/logrus"
	"html/template"
	"io"
	"net/http"
	"os"
	"strings"
)

// tAdd gotemplate function to subtract some values
func (s *Server) tSub(vals ...int) int {
	v := 0
	for i, vi := range vals {
		if i == 0 {
			v = vi
		} else {
			v -= vi
		}
	}
	return v
}

// tTranslationsJoin gotemplate function to join translations and alternatives
func (s *Server) tTranslationsJoin(res *translate.Response) string {
	if res == nil {
		return ""
	}
	alts := translate.CanonicalizeTranslations(res)

	return strings.Join(alts, "\n")
}

// tFavAltsJoin gotemplate function to join favorites
func (s *Server) tFavAltsJoin(fav favorites.Favorite) string {
	return strings.Join(fav.Target, "\n")
}

// tLangIsChecked gotemplate function to return a safe string that can be added as a html attribute
func (s *Server) tLangIsChecked(lang string, inLang string) template.HTMLAttr {
	var attr template.HTMLAttr
	if lang == inLang {
		attr = template.HTMLAttr(`checked="checked"`)
	}
	return attr
}

// tJsonTranslateData provide a translation response as a JSON  URL-encoded string
func (s *Server) tJsonTranslateData(res *TranslateResponse) string {
	data, err := json.Marshal(res)
	if err != nil {
		return ""
	}
	return string(data)
}

func (s *Server) LoadTemplates() error {
	dir := s.cfg.StaticPages + "/templates"
	t, err := template.New("template/").Funcs(template.FuncMap{
		"sub":               s.tSub,
		"trjoin":            s.tTranslationsJoin,
		"favAltsJoin":       s.tFavAltsJoin,
		"langIsChecked":     s.tLangIsChecked,
		"jsonTranslateData": s.tJsonTranslateData,
	}).ParseGlob(dir + "/*.gohtml")
	if err != nil {
		return err
	}
	s.templates = t

	return nil
}

func (s *Server) renderTemplate(w http.ResponseWriter, name string, templData any) error {
	var rerr error
	defer func() {
		if rerr != nil {
			logrus.WithError(rerr).Error("renderTemplate failed")
		}
	}()

	logrus.WithFields(logrus.Fields{
		"name":      name,
		"templData": templData != nil,
	}).Debug("renderTemplate")

	err := s.templates.ExecuteTemplate(w, name, templData)
	if err != nil {
		if !strings.HasSuffix(name, ".gohtml") {
			p := name
			if name[0] != '/' {
				p = "/" + name
			}
			fullPath := s.cfg.StaticPages + p

			fp, err := os.Open(fullPath)
			if err != nil {
				return err
			}
			_, _ = io.Copy(w, fp)
			_ = fp.Close()
			return nil
		} else {
			logrus.WithField("template", name).WithError(err).Error("template failure")
		}

		return err
	}

	logrus.WithField("name", name).Debug("rendered template")
	return nil
}
