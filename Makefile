.PHONY: build

build: ## 打包到 pkg/jh_gateway
	mkdir -p pkg
	go build -trimpath -ldflags="-s -w" -o pkg/jh_gateway .
