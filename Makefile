.PHONY: tidy
tidy:
	go mod tidy

.PHONY: run
run:
	go run ./

.PHONY: build
build:
	go build -o ./bin/proxy ./

.PHONY: docker
docker:
	docker build -t proxy .

.PHONY: docker-run
docker-run:
	docker run -p 8080:8080 --name proxy -t proxy
