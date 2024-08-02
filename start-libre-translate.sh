#!/bin/sh

# start API on localhost:6001
libretranslate --port 6001 --load-only en,es --disable-web-ui --update-models
