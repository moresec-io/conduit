all: build

build:
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o conduit cmd/main.go

clean:
	rm conduit

output: build
