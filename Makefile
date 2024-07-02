all: build

build:
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o release/bin/conduit cmd/agent/main.go

clean:
	rm conduit

output: build
