SHELL:=/usr/bin/env bash

include .env
export $(shell sed 's/=.*//' .env)

DOCKER_REGISTRY:=docker.io/mikmuellerdev

REEF_CADDY_IMAGE_TAG:=reef_caddy
REEF_MANAGER_IMAGE_TAG:=reef_manager
REEF_COMPILER_IMAGE:=reef_caddy
CONTAINER_TAGS:="$(REEF_CADDY_IMAGE_TAG) $(REEF_MANAGER_IMAGE_TAG) $(REEF_COMPILER_IMAGE)"

.PHONY: build_containers push_containers
build-containers:
	for image in "$(CONTAINER_TAGS)"; do \
		echo "Building '$$image'"; \
		nix build ".#$${image}_image" && ./result | docker load ; \
		echo "Renaming '$$image' to $(DOCKER_REGISTRY)/$${image}"; \
		docker tag "$${image}" "$(DOCKER_REGISTRY)/$${image}" ; \
	done

push-containers:
	for image in "$(CONTAINER_TAGS)"; do \
		echo "Pushing image: '$(DOCKER_REGISTRY)/$$image'"; \
		docker push "$(DOCKER_REGISTRY)/$$image"; \
	done

PORT=3000

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
