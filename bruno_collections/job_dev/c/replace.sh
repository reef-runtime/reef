#!/usr/bin/env bash

OUTP=$1
DATASET=$2

sed '1d' input.c > input.c.tr

sanitized=$(jq -Rs . input.c.tr)

echo "Removing '${OUTP}'..."

rm -f "$OUTP"

echo "Creating '${OUTP}'..."

printf '%s' "$(<./req_templ0)" >> "$OUTP"
echo -n "${sanitized}" >> "$OUTP"
cat ./req_templ1 >> "$OUTP"

# Replace dataset placeholder with real ID.
sed -i -e "s/DATASET_ID_PLACEHOLDER/${DATASET}/g" "$OUTP"
