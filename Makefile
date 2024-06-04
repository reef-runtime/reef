.PHONY: test

test:
	cd ./reef_manager/ && make test && make lint
	cd ./reef_protocol/ && make test
	typos .
