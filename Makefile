all: conduit

.PHONY: conduit
conduit:
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o release/bin/conduit cmd/conduit/main.go

.PHONY: manager
manager:
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o release/bin/manager cmd/manager/main.go

.PHONY: cert
cert:
	mkdir -p cert && cd cert && sh ../dist/scripts/gen_cert.sh && cd -

clean:
	rm release/bin/conduit
	rm release/bin/manager

output: build
