SHELL:=/usr/bin/env bash

include .env
export $(shell sed 's/=.*//' .env)

.PHONY: build_containers
build_containers:
	nix build .#reef_caddy_image && ./result | docker load
	nix build .#reef_manager_image && ./result | docker load
	nix build .#reef_compiler_image && ./result | docker load

PORT="3000"

.PHONY: up down
up:
	PORT=$(PORT) docker-compose up
down:
	PORT=$(PORT) docker-compose down

.PHONY: test
test:
	cd ./reef_manager/ && make test && make lint
	cd ./reef_protocol/ && make test
	typos .

run:
	cd ./reef_compiler/ && make run &
	sleep 1
	cd ./reef_manager/ && make run &
	sleep 1
	cd ./reef_frontend/ && pnpm run dev &
