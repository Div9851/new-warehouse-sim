.PHONY: build clean

GO_FILES:=$(shell find . -type f -name '*.go' -print)

build: main

main: $(GO_FILES)
	go build -o main cmd/main.go

clean:
	rm -f main
