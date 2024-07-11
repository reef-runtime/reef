SHELL:=/usr/bin/env bash -o pipefail

include .env
export $(shell sed 's/=.*//' .env)

NIX_ARGS:=

PORT=3000
.PHONY: up down build-containers push-containers test run

#
# Note: change this to something like `registry.tld/username` in order to push to a private registry.
#

DOCKER_REGISTRY:=docker.io/mikmuellerdev


#
# Predefined component names, do not touch these.
#

REEF_CADDY_IMAGE_TAG:=reef_caddy
REEF_MANAGER_IMAGE_TAG:=reef_manager
REEF_COMPILER_IMAGE:=reef_compiler
CONTAINER_TAGS:="$(REEF_CADDY_IMAGE_TAG) $(REEF_MANAGER_IMAGE_TAG) $(REEF_COMPILER_IMAGE)"

#
# Builds all predefined containers.
#

build-containers:
	echo "$(CONTAINER_TAGS)"
	for image in "$(CONTAINER_TAGS)"; do \
		echo "Building '$$image'" && \
		nix $(NIX_ARGS) build ".#$${image}_image" && ./result | docker load && \
		echo "Renaming '$$image' to $(DOCKER_REGISTRY)/$${image}"&& \
		docker tag "$${image}" "$(DOCKER_REGISTRY)/$${image}" || exit 1; \
	done

#
# Pushes all previously built containers to `$DOCKER_REGISTRY`
#

push-containers:
	echo "$(CONTAINER_TAGS)"
	for image in "$(CONTAINER_TAGS)"; do \
		echo "Pushing image: '$(DOCKER_REGISTRY)/$$image'" && \
		docker push "$(DOCKER_REGISTRY)/$$image" || exit 1; \
	done

#
# Enabling / Disabling the entire stack.
#

up:
	PORT=$(PORT) docker-compose up
down:
	PORT=$(PORT) docker-compose down

#
# Testing and linting.
#

test:
	cd ./reef_manager/ && make test && make lint
	cd ./reef_protocol/ && make test
	typos .
