# Spanish Translation using free API
This is an app to translate to/from english->spanish using the free [Argos Translate python package](https://github.com/argosopentech/argos-translate/tree/master) and to store favorite translations into a local sqlite3 database. Everything is hosted locally.

The web server is built using TailwindCSS and HTMX.

> WARNING: there is no authentication for this app. It is a utility intended to be run on localhost or a restricted network. No PII data is stored.

## Caveat
There are MUCH better translation solutions online. I highly recommend [DeepL](https://www.deepl.com/en/translator). It has many free features, but you need to pay to store favorite lists. If you are looking for a single tool to translate, speak, integrate with your OS, et al., then DeepL is for you.

Otherwise, if you are cheap like me, then this project may be what you are looking for.

## Quickstart
* when run, the directory of this code base will also contain the DB file
* install Argos Translate python package (see their github README)
* Start the web server
  * > go run cmd/server/main.go -favorites-dburl file://$PWD/favorites.db
* Browse to http://localhost:8080

This starts the web server on port 8080 and the Argos REST service on port 6001. Both may be changed using command-line options.

## Argos Translate
Ref: [Argos Translate python package](https://github.com/argosopentech/argos-translate/tree/master)

This is a large python package that provides libraries to translate to/from various languages. Initially you must download it. 

A [local script](./argostranslate-api.py) starts an API server that minimally responds like LibreTranslate. The main web server manages the running of the script. This was designed to avoid the LibreTranslate external dependency, so only a single app need be started.

## Optional, Command-line Usage
The command-line app is more of a simple convenience.
> make
```shell
Usage of ./esptrans:
  -debug string
    	 (default "INFO")
  -favorites-dburl string
    	Favorites DB URL (default "file:///.../favorites.db" (in local directory))
  -libre-translate-url string
    	Libre Translate URL (default "http://localhost:6001")
  -n	Do not save to favorites
  -r	Translate es=>en. Default is inverse (en=>es).
  -v	Verbose output
```

## Resources
### LibreTranslate API
**Request**
```javascript
const res = await fetch("https://libretranslate.com/translate", {
	method: "POST",
	body: JSON.stringify({
		q: "I had scheduled a class for 9:00am and recieved an email at 9 that a class would start at 9:30. Is that common to be given such short notice?",
		source: "auto",
		target: "es",
		format: "text",
		alternatives: 3,
		api_key: ""
	}),
	headers: { "Content-Type": "application/json" }
});

console.log(await res.json());
```

**Response**
```javascript
{
    "alternatives": [
        "Había programado una clase para las 9:00 a.m. y recibí un email a las 9 que una clase comenzaría a las 9:30. ¿Es común que se le dé tan poco aviso?",
        "Había programado una clase para las 9:00 a.m. y recibí un correo electrónico a las 9 que una clase empezaría a las 9:30. ¿Eso es común que se le dé tan breve aviso?",
        "Había programado una clase para las 9:00 a.m. y reconocí un email a las 9 que una clase comenzaría a las 9:30. ¿Es eso común que se le dé tan breve aviso?"
    ],
    "detectedLanguage": {
        "confidence": 100,
        "language": "en"
    },
    "translatedText": "Había programado una clase para las 9:00 a.m. y recibí un correo electrónico a las 9 que una clase comenzaría a las 9:30. ¿Es común que se le dé tan breve aviso?"
}
```