

.PHONY: test coverage fuzz fmt

test:
	go clean -testcache && go test ./... -race -timeout 30s

coverage:
	-go clean -testcache && go test ./... -timeout 30s -race -covermode=atomic -coverprofile=coverage.txt
	rm ./coverage.txt

fuzz_prefixfs:
	gotip clean -testcache && gotip test -fuzz=FuzzPrefixFs -race -fuzztime=300s

fuzz_hiddenfs:
	gotip clean -testcache && gotip test -fuzz=FuzzHiddenFsCreate -race -fuzztime=300s

fmt:
	go fmt ./...
