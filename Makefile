BINARY := $(HOME)/bin/overseer
CLI    := ./cli

.PHONY: dev clean

## dev: build and install overseer locally (no tag needed)
dev:
	go build -C $(CLI) -o $(BINARY) .
	@echo "installed $(BINARY)"

## clean: remove the local binary
clean:
	rm -f $(BINARY)
