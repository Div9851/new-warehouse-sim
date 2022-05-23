.PHONY: build clean

GO_FILES:=$(shell find . -type f -name '*.go' -print)

build: sim

sim: $(GO_FILES)
	go build -o sim cmd/main.go

clean:
	rm -f sim
