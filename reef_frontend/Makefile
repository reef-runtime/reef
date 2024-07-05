#!make
SHELL:=/usr/bin/env bash

ENV_FILE=../.env
include $(ENV_FILE)
export $(shell sed 's/=.*//' $(ENV_FILE))

run:
	npm install
	PORT=$(REEF_NEXTJS_PORT) npm run dev

.PHONY: build-wasm
build-wasm:
	rm -rf ./src/lib/node_web_generated
	nix build .#reef_node_web
	cp -r ./result/pkg/ ./src/lib/node_web_generated
	chmod -R +w ./src/lib/node_web_generated
