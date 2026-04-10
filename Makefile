BINARY := $(HOME)/bin/overseer
CLI    := ./cli
DOCS   := ./docs

.PHONY: dev clean docs

## dev: build and install overseer locally (no tag needed)
dev:
	go build -C $(CLI) -o $(BINARY) .
	@echo "installed $(BINARY)"

## docs: serve the documentation site locally with live reload
docs:
	hugo server -s $(DOCS) --bind 0.0.0.0 --port 1313

## clean: remove the local binary
clean:
	rm -f $(BINARY)
