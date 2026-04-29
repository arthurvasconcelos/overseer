binary := home_dir() / "bin/overseer"
cli    := "./cli"
docs   := "./docs"

# build and install overseer locally (no tag needed)
dev:
    go build -C {{cli}} -o {{binary}} .
    @echo "installed {{binary}}"

# serve the documentation site locally with live reload
docs:
    hugo server -s {{docs}} --bind 0.0.0.0 --port 1313

# remove the local binary
clean:
    rm -f {{binary}}
