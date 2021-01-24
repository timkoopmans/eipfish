.PHONY: build clean deploy

build:
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/handler -a -ldflags '-s -w -extldflags "-static"' handler.go

clean:
	rm -rf ./bin ./vendor Gopkg.lock

deploy: clean build
	sls deploy --verbose

destroy:
	sls remove

invoke:
	sls invoke -f fish --data '{"region": "ap-southeast-2"}'

test:
	SLS_DEBUG=* sls invoke local -f fish --data '{"region": "ap-southeast-2"}'
