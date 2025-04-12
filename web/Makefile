TSC ?= tsgo

default: run

server.elf: server/main.go
	go build -o $@ ./server

dist/wiwali.wasm:
	@mkdir -p dist
	GOOS=js GOARCH=wasm go build -o dist/wiwali.wasm ./wiwali

dist/wasm_exec.js:
	@mkdir -p dist
	cp "$$(go env GOROOT)/misc/wasm/wasm_exec.js" dist/wasm_exec.js

node_modules: package.json yarn.lock
	yarn

check: node_modules
	$(TSC) -noEmit -p tsconfig.json | tee check.log
	@ ! grep -q error check.log

dist/components.js: $(wildcard src/*.ts) check node_modules
	@mkdir -p dist
	esbuild --bundle --minify --sourcemap --target=es2020 src/components.ts > $@

dist/index.html: index.html
	cp $< $@

dist/edk2.wasm: ../pkg/efi/compression/edk2.wasm
	cp $< $@

dist/favicon.png: favicon.png
	cp $< $@

dist: dist/wiwali.wasm dist/wasm_exec.js dist/components.js dist/index.html dist/edk2.wasm dist/favicon.png

deploy: dist
	$(if $(HOST),,$(error HOST must be set))
	rsync -ayvz dist/ $(HOST):/var/www/nugget.zone/

run: dist server.elf
	./server.elf -root dist

clean:
	rm -rf dist check.log node_modules server.elf

.PHONY: dist/wiwali.wasm clean check dist deploy
