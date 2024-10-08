#!make
SHELL:=/usr/bin/env bash -o pipefail

#
# Reef manager Makefile.
# Authors: MikMuellerDev, konstantin-ebeling
# This file is part of the reef project and is used to make
# development work and deployment easier.
#

ENV_FILE=.env

include $(ENV_FILE)
export $(shell sed 's/=.*//' $(ENV_FILE))

################################### VARS / SETUP ####################################

.PHONY: up down build-containers push-containers test run
.NOTPARALLEL: up down build-containers push-containers test run

#
# Misc.
#

ENV_PREFIX=REEF
GREP:=rg
TYPOS:=typos

#
# Nix arguments can be passed down to control Nix invocations.
#

NIX:=nix
NIX_ARGS:=

#
# When using the docker-compose deployment target,
# this is the part on which the production stack runs.
#

PORT=3000

#
# Note: change this to something like `registry.tld/username`
# in order to push to a private registry.
#

DOCKER_REGISTRY:=docker.io/mikmuellerdev
DOCKER:=docker
DOCKER_COMPOSE:=docker-compose

#
# Configuration for automatic cloning of example jobs.
#

GIT=git
EXAMPLES_REPO=https://github.com/reef-runtime/examples
LOCAL_EXAMPLES_DIR=reef_templates


#
# Predefined component names, do not modify these.
#

REEF_CADDY_IMAGE_TAG:=reef_caddy
REEF_MANAGER_IMAGE_TAG:=reef_manager
REEF_COMPILER_IMAGE:=reef_compiler
REEF_NODE_NATIVE_IMAGE:=reef_node_native
CONTAINER_TAGS:="$(REEF_CADDY_IMAGE_TAG) $(REEF_MANAGER_IMAGE_TAG) $(REEF_COMPILER_IMAGE) $(REEF_NODE_NATIVE_IMAGE)"

################################### BEGIN TARGETS ##################################

#
# Builds all predefined containers.
#

build-containers: env
	echo "$(CONTAINER_TAGS)"
	for image in "$(CONTAINER_TAGS)"; do \
		echo "Building '$$image'" && \
		$(NIX) $(NIX_ARGS) build ".#$${image}_image" && ./result | $(DOCKER) load || exit 1; \
	done

#
# Pushes all previously built containers to `$DOCKER_REGISTRY`.
#

push-containers: env
	echo "$(CONTAINER_TAGS)"
	for image in "$(CONTAINER_TAGS)"; do \
		echo "Renaming '$$image' to $(DOCKER_REGISTRY)/$${image}" && \
		$(DOCKER) tag "$${image}" "$(DOCKER_REGISTRY)/$${image}" && \
		echo "Pushing image: '$(DOCKER_REGISTRY)/$$image'" && \
		$(DOCKER) push "$(DOCKER_REGISTRY)/$$image" || exit 1; \
	done

#
# Enabling / Disabling the entire stack.
#

up:
	if [ ! -d $(LOCAL_EXAMPLES_DIR) ]; then \
		if [ $(EXAMPLES_REPO) = "" ]; then \
			echo "NOTE: no example repository specified, skipping local clone."; \
		else \
			echo "Cloning job examples..."; \
			$(GIT) clone $(EXAMPLES_REPO) $(LOCAL_EXAMPLES_DIR); \
			echo "Cloned remote job examples locally"; \
		fi \
	else \
		echo "Updating job examples..."; \
		cd $(LOCAL_EXAMPLES_DIR) && $(GIT) pull; \
		echo "Updated local job examples"; \
	fi \

	PORT=$(PORT) $(DOCKER_COMPOSE) up
down:
	PORT=$(PORT) $(DOCKER_COMPOSE) down

#
# Testing and linting.
#

test:
	cd ./reef_manager/ && make test && make lint
	cd ./reef_protocol/ && make test
	$(TYPOS) .

env: $(ENV_FILE)
	env | $(GREP) $(ENV_PREFIX)
