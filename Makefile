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
