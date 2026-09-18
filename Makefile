.PHONY: build agent api ui demo clean install

GO ?= go
export PATH := $(CURDIR)/.tools/go/bin:$(PATH)

build: agent api

agent:
	$(GO) build -o bin/hebernet-agent ./cmd/agent

api:
	$(GO) build -o bin/hebernet-api ./cmd/api

ui:
	cd ui && npm install && npm run build

demo:
	./scripts/demo.sh

install:
	./scripts/install.sh

clean:
	rm -rf bin/ data/*.db data/*.sock data/homes data/nginx data/php-fpm data/users data/quotas data/ssl data/mysql data/suspended
