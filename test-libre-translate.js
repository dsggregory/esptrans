// node test-libre-translate.js
async function doit() {
	const res = await fetch("http://localhost:6001/translate"/*"https://libretranslate.com/translate"*/, {
		method: "POST",
		body: JSON.stringify({
			q: "This is a test of my local installation.",
			source: "auto",
			target: "es",
			format: "text",
			alternatives: 3,
			api_key: ""
		}),
		headers: { "Content-Type": "application/json" }
	});

	console.log(await res.json());

}

doit()