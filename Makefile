.PHONY: help server test convert

INPUT ?= testdata/battle-report
OUTPUT_DIR ?= testdata/analyze_request

help:
	@echo "make server                 Start the HTTP server"
	@echo "make test                   Run go tests"
	@echo "make convert                Batch convert battle reports"
	@echo "make convert INPUT=... OUTPUT_DIR=..."

server:
	go run ./cmd/server

test:
	go test ./...

convert:
	go run ./cmd/convertbatch -input $(INPUT) -output-dir $(OUTPUT_DIR)
