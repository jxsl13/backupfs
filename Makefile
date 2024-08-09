

.PHONY: test coverage fuzz fmt

test:
	go clean -testcache && go test ./... -count=1 -race -timeout 30s

coverage:
	-go clean -testcache && go test ./... -count=1 -timeout 30s -race -covermode=atomic -coverprofile=coverage.txt
	rm ./coverage.txt

fuzz_prefixfs:
	go clean -testcache && go test -fuzz=FuzzPrefixFS -race -fuzztime=300s

fuzz_hiddenfs_create:
	go clean -testcache && go test -fuzz=FuzzHiddenFSCreate -race -fuzztime=300s

fuzz_hiddenfs_remove_all:
	go clean -testcache && go test -fuzz=FuzzHiddenFSRemoveAll -race -fuzztime=300s

fuzz_sort_by_most:
	go clean -testcache && go test -fuzz=FuzzSortByMostFilePathSeparators -race -fuzztime=300s

fuzz_sort_by_least:
	go clean -testcache && go test -fuzz=FuzzSortByLeastFilePathSeparators -race -fuzztime=300s


fmt:
	go fmt ./...

gen_mock:
	go generate ./...

syntax:
	GOOS=windows go build ./...
	GOOS=linux go build ./...
	GOOS=darwin go build ./...
