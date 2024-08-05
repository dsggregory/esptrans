# Spanish Translation using free API
This is an app to translate input into/out-of english->spanish using the free [Libre Translate](https://libretranslate.com) and to store favorite translations into a local sqlite3 database.

It costs to use LibreTranslate API services, but you can host [your own server](https://github.com/LibreTranslate/LibreTranslate). This is what we'll be doing.

## Caveat
There are MUCH better translation apps online. I highly recommend [DeepL](https://www.deepl.com/en/translator). It has many free features, but you need to pay to store favorite lists. If you are looking for a single tool to translate, speak, integrate with your OS, et al., then DeepL is for you.

Otherwise, if you are cheap like me, then this project may be what you are looking for.

## LibreTranslate
### Install LibreTranslate
LibreTranslate requires python3.
```shell
pip install libretranslate
```

### Startup LibreTranslate self-hosted server
```shell
sh ./start-libre-translate.sh &
```
Test that it works by running:
> node test-libre-translate.js

## Usage
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