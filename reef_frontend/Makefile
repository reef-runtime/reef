#!make
SHELL:=/usr/bin/env bash

ENV_FILE=../.env
include $(ENV_FILE)
export $(shell sed 's/=.*//' $(ENV_FILE))

run:
	npm install
	PORT=$(REEF_NEXTJS_PORT) npm run dev

.PHONY: build-rust
build-rust:
	rm -rf ./src/lib/node_web_generated
	nix build .#reef_node_web
	cp -r ./result/pkg/ ./src/lib/node_web_generated
	chmod -R +w ./src/lib/node_web_generated
	rm -f ./public/reef_node_native
	nix build .#reef_node_native
	cp ./result/bin/reef_node_native ./public
	chmod +w ./public/reef_node_native
