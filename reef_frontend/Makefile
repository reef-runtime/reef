#!make
SHELL:=/usr/bin/env bash

ENV_FILE=../.env
include $(ENV_FILE)
export $(shell sed 's/=.*//' $(ENV_FILE))

run:
	npm install
	PORT=$(REEF_NEXTJS_PORT)
	npm run dev

