#!make
SHELL:=/bin/bash
export BASH_ENV=./env

PROJECT_NAME="reef"

.PHONY: dev-db-down dev-db-up dev-db-nuke lint
.NOTPARALLEL: dev-db-up dev-db-down dev-db-nuke
GREP:=grep

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

GO_TEST_BIN:=richgo
GO_TEST_ARGS:=test -v -p 1 ./... -timeout=100000s

lint: *.go ./logic ./database/ ./api/
	$(GO_TEST_BIN) $(GO_TEST_ARGS)
