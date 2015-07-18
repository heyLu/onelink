build:
	go get -v -d ./...
	go build onelink.go

release: build
	mkdir -p /tmp/onelink
	cp -Rv onelink etc schema.edn init.edn /tmp/onelink
	tar -czf onelink.tar.gz --transform='s/tmp\///' /tmp/onelink
	rm -rf /tmp/onelink
