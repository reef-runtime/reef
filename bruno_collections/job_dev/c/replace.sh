#!/usr/bin/env bash

OUTP=$1

sed '1d' input.c > input.c.tr

sanitized=$(jq -Rs . input.c.tr)

echo "Removing '${OUTP}'..."

rm -f "$OUTP"

echo "Creating '${OUTP}'..."

printf '%s' "$(<./req_templ0)" >> "$OUTP"
echo -n "${sanitized}" >> "$OUTP"
cat ./req_templ1 >> "$OUTP"
