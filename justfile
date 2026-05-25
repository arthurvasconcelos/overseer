binary := home_dir() / "bin/overseer"
cli    := "./cli"
docs   := "./docs"

# build and install overseer locally (no tag needed)
dev:
    go build -C {{cli}} -o {{binary}} .
    @echo "installed {{binary}}"

# serve the documentation site locally with live reload
docs:
    pnpm docs:dev

# remove the local binary
clean:
    rm -f {{binary}}
