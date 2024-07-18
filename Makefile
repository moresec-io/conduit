all: build

build:
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o release/bin/conduit cmd/conduit/main.go

.PHONY: cert
cert:
	mkdir -p cert && cd cert && sh ../dist/scripts/gen_cert.sh && cd -

clean:
	rm conduit

output: build
