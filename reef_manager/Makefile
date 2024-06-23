#!make
SHELL:=/usr/bin/env bash

ENV_FILE=../.env
include $(ENV_FILE)
export $(shell sed 's/=.*//' $(ENV_FILE))

################################### VARS / SETUP ####################################

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


MIGRATE:=migrate
MIGRATIONS_DIR:=db/migrations

#
# Caddy & web
#

CADDY_BIN=caddy

#
# Go stuff.
#

GO_LINT_BIN:=golangci-lint
GO_FLAGS:=-v -race
GO_BIN:=go
GO_TEST_BIN:=richgo
GO_TEST_ARGS:=test -v -p 1 ./... -timeout=100000s
GO_SRC:=$(wildcard *.go) $(wildcard ./logic/**/*.go) $(wildcard ./database/**/*.go) $(wildcard ./api/**/*.go)

################################### BEGIN TARGETS ##################################

#
# Database targets
#

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

MIGRATION_NAME = "<error:not-specified>"
dev-db-add-migration:
	$(MIGRATE) create -ext sql -dir $(MIGRATIONS_DIR) -seq $(MIGRATION_NAME)

dev-db-migrate-run:
	$(MIGRATE) -database ${POSTGRESQL_URL} -path $(MIGRATIONS_DIR) up

#
# Server / GO.
#

lint: $(GO_SRC)
	$(GO_LINT_BIN) run

test: $(GO_SRC)
	$(GO_TEST_BIN) $(GO_TEST_ARGS)

run: $(GO_SRC)
	$(CADDY_BIN) run &
	$(GO_BIN) mod tidy
	$(GO_BIN) run $(GO_FLAGS) .


#
# Misc.
#

clean:
	rm -rf log
	rm -f $(DOCKER_OK_FILE)

env: $(ENV_FILE)
	env | $(GREP) REEF
	env | $(GREP) POSTGRESQL_URL
