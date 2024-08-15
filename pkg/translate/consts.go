package translate

const (
	English = "en"
	Spanish = "es"
	Any     = "auto"
)

// Request the struct to send as a translation request
type Request struct {
	Q            string `json:"q"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	Format       string `json:"format"`
	Alternatives int    `json:"alternatives"`
	APIKey       string `json:"api_key"`
}

// Response the struct received by the translation service
type Response struct {
	Input            string   `json:"input,omitempty"`
	Alternatives     []string `json:"alternatives"`
	DetectedLanguage struct {
		Language string `json:"language"`
		// Confidence represents quality of the translation. Per argostranslate, 0=no-confidence, <0=more-confidence.
		Confidence float64 `json:"confidence"`
	} `json:"detectedLanguage"`
	TranslatedText string `json:"translatedText"`
}
