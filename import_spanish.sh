#!/bin/sh
# Import spanish words or phrases - one per line entered at console

while read -r line; do
  ./esptrans -r -v "$line"
done
