# A frontend to the argostranslate python package that is called by esptrans. Esptrans will maintain it running.
# This is our implementation of LibreTranslate API server for just the endpoints we need.

# Request=> {Q:str,Source:fm_code,Target:to_code,Alternates:n}\nEOF\n
#      ex: {"Q":"hello","Source":"en","Target":"es","Alternatives":3}
# Response=> {Input: srctext, Alternatives: []str, DetectedLanguage: {Language:str,Confidence:float}, TranslatedText: str}
#      ex: {"Input": "hello", "DetectedLanguage": {"Language": "en"}, "TranslatedText": "Hola.", "Alternatives": ["Hola", "hola", "hola."]
#
# https://github.com/argosopentech/argos-translate/releases/download/v1.4.0/argostranslate.app.zip
# pip install argostranslate
from sys import stdin
import argostranslate.package
import argostranslate.translate
import json
from http.server import BaseHTTPRequestHandler, HTTPServer
import time
import argparse

class APIServer(BaseHTTPRequestHandler):
    def do_POST(self):
        try:
            content_len = int(self.headers.get('Content-Length'))
            post_body = self.rfile.read(content_len)
            req = json.loads(post_body)

            resp = argos.translate(req)
            respJson = json.dumps(resp)
        except Exception as e:
            self.send_response(500)
            self.send_header("Content-type", "text/plain")
            self.end_headers()
            self.wfile.write(e)
            print(e)

        self.send_response(200)
        self.send_header("Content-type", "text/json; charset=utf-8")
        self.end_headers()
        self.wfile.write(respJson.encode('utf-8'))

class CmdLineProto():
    EOJ = "EOJ\n"

    def handle(self, argos):
        print('''    #Protocol
                   # READY\\n       - ready to read from stdin until "EOJ" on line by itself
                   # request       - caller sends json object of Request type
                   # EOJ\\n         - caller sends this on line by itself to indicate preceding line(s) to be translated
                   # ACCEPTED nBytes   - translation was successful. nBytes is number of bytes to read for Response.
                   # REJECTED <.error> - problem with the request
        ''')

        # loop forever
        print("READY")
        data=''
        while 1==1:
            try:
                req = self.readJsonData()
                if req == None:
                    break
            except Exception as e:
                print("REJECTED " + e)
                continue

            resp = argos.translate(req)

            respJson = json.dumps(resp)
            l = len(respJson) + 1 # +1 for the newline that print will add
            print("ACCEPTED %d" % l)
            print(respJson)

    def readJsonData(self):
        jsonData = ''
        for line in stdin:
            if line == self.EOJ:
                break
            jsonData += line
        if len(jsonData) == 0:   # got Ctrl-D (real EOF on stdin)
            return None
        return json.loads(jsonData)

class Argos:
    def __init__(self):
        self.from_code = "en"
        self.to_code = "es"

        # Download and install Argos Translate packages
        argostranslate.package.update_package_index()
        available_packages = argostranslate.package.get_available_packages()
        self.dnld(available_packages, self.from_code, self.to_code)
        self.dnld(available_packages, self.to_code, self.from_code)

    def translate(self, req):
        # Translate
        tr = argostranslate.translate.get_translation_from_codes(req['source'], req['target'])
        hypotheses = tr.hypotheses(req['q'])

        nAlts = req['alternatives'] + 1
        hs = []
        for x in hypotheses:
            hs.append(x.value)
        alts = hs[1:nAlts] # remove what we are using for TranslatedText (the best score) and any extras
        resp = {
            "input": req['q'],
            "detectedLanguage": {"Language": req['source']},
            "translatedText": hypotheses[0].value,
            "alternatives": alts
        }

        return resp

    def dnld(self, avail, fromcd, tocd):
        package_to_install = next(
            filter(
                lambda x: x.from_code == fromcd and x.to_code == tocd, avail
            )
        )
        argostranslate.package.install_from_path(package_to_install.download())

def webServer(argos, listen):
    la = listen.split(':')
    webServer = HTTPServer((la[0], int(la[1])), APIServer)
    print("Server started http://%s" % listen)

    try:
        webServer.serve_forever()
    except KeyboardInterrupt:
        print('interrupt')

    webServer.server_close()
    print("Server stopped.")

def cmdLine(argos):
    c = CmdLineProto()
    c.handle(argos)

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--listen", type=str, default="localhost:6001", help="start the REST API server")
    parser.add_argument("--console", action='store_true', help="translate from stdin")
    args = parser.parse_args()

    argos = Argos()

    if args.console == False:
        webServer(argos, args.listen)
    else:
        cmdLine(argos)