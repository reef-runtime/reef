.PHONY: test

test:
	cd ./reef_manager/ && make test
	cd ./reef_protocol/ && make test
	typos .
