all: build

build:
	go build -ldflags "-s -w" -o conduit cmd/main.go

clean:
	rm conduit

output: build
