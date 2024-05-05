#!make
SHELL:=/usr/bin/env bash

include .env
export $(shell sed 's/=.*//' .env)

PROJECT_NAME="reef"
GREP:=rg

.PHONY: dev-db-down dev-db-up dev-db-nuke lint clean
.NOTPARALLEL: dev-db-up dev-db-down dev-db-nuke

#
# (Dev) database specific.
#

DOCKER:=docker
DOCKER_COMPOSE:=docker-compose
DOCKER_COMPOSE_DEV_FILE:=./docker-compose-dev.yml
DOCKER_DEV_DB_VOLUME_NAME:=$(PROJECT_NAME)_manager_$(PROJECT_NAME)-db-data

# Required for docker testing.
DOCKER_OK_IMAGE:=bash
DOCKER_OK_FILE:=.docker_ok
DOCKER_OK_ERR_MSG:=Is your docker setup working?


$(DOCKER_OK_FILE):
	$(DOCKER) run $(DOCKER_OK_IMAGE) echo 'OK' > $(DOCKER_OK_FILE)
	cat $(DOCKER_OK_FILE) | $(GREP) 'OK' || ( echo $(DOCKER_OK_ERR_MSG); exit 1 )

docker-check: $(DOCKER_OK_FILE)

dev-db-up: docker-check
	@echo "Starting dev database..."
	$(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_DEV_FILE) up  -d

dev-db-down: docker-check
	@echo "Tearing down dev database..."
	$(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_DEV_FILE) down
	docker volume rm $(DOCKER_DEV_DB_VOLUME_NAME) || ( echo "[Not running?] Dev database volume does not exist" )

dev-db-nuke: dev-db-down dev-db-up

dev-db-shell: docker-check
	$(DOCKER) exec \
		-e "PGPASSWORD:$(REEF_DB_PASSWORD)" \
		-it $(PROJECT_NAME)-db \
		psql \
			--username $(REEF_DB_USERNAME) \
			-d $(REEF_DB_NAME)


# TODO: add target for simplified DB migrations

#
# Server / GO.
#

GO_FLAGS:=-v -race
GO_BIN:=go
GO_TEST_BIN:=richgo
GO_TEST_ARGS:=test -v -p 1 ./... -timeout=100000s
GO_SRC:=$(wildcard *.go) $(wildcard ./logic/**/*.go) $(wildcard ./database/**/*.go) $(wildcard ./api/**/*.go)

lint: $(GO_SRC)
	$(GO_TEST_BIN) $(GO_TEST_ARGS)

run: $(GO_SRC)
	$(GO_BIN) run $(GO_FLAGS) .

#
# Misc.
#

clean:
	rm -rf log
	rm -f $(DOCKER_OK_FILE)

env: .env
	env | $(GREP) REEF
