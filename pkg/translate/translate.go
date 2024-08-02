package translate

import (
	"esptrans/pkg/favorites"
	"esptrans/pkg/libre_translate"
	"fmt"
	"strings"
)

type TranslateOptions struct {
	InLang  string
	OutLang string
	DB      *favorites.DBService
	LT      *libre_translate.LTClient
}

func canonicalizeString(s string) string {
	// lowercase if not a phrase (w/out punctuation)
	if !strings.ContainsAny(s, "?.!") {
		s = strings.ToLower(s)
	}
	// Fixup input text - removes sp, nl, quotes
	s = strings.Trim(s, " \"\r\n")

	return s
}

func saveFavorite(opts *TranslateOptions, source string, res *libre_translate.Response) error {
	if opts.DB != nil {
		// use a map to avoid dups and maintain order in resulting array
		malts := make(map[string]bool)
		malts[res.TranslatedText] = true
		alts := []string{res.TranslatedText}
		for _, x := range res.Alternatives {
			if _, ok := malts[x]; !ok {
				alts = append(alts, x)
			}
			malts[x] = true
		}
		fav := favorites.Favorite{
			Source:     source,
			Target:     alts,
			SourceLang: opts.InLang,
			TargetLang: opts.OutLang,
		}
		if res.DetectedLanguage.Language != "" {
			fav.SourceLang = res.DetectedLanguage.Language
		}
		_, err := opts.DB.AddFavorite(&fav)
		if err != nil {
			if !strings.Contains(err.Error(), "UNIQUE") {
				return fmt.Errorf("error adding favorite: %w", err)
			}
		}
	}
	return nil
}

// Translate calls the LibreTranslate wrapper and saves to favorites
func Translate(opts *TranslateOptions, sdata string) (*libre_translate.Response, error) {
	if len(sdata) == 0 {
		return nil, fmt.Errorf("empty string")
	}
	sdata = canonicalizeString(sdata)

	res, err := opts.LT.Translate(sdata, opts.InLang, opts.OutLang)
	if err != nil {
		return nil, fmt.Errorf("Failed to translate: %w", err)
	}

	if err = saveFavorite(opts, sdata, res); err != nil {
		return nil, err
	}
	return res, nil
}
