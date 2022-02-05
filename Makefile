

test:
	go clean -testcache && go test -race -timeout 30s ./...

coverage:
	go clean -testcache && go test -timeout 30s -race -coverprofile=./coverage ./...
	rm ./coverage

fuzz:
	gotip clean -testcache && gotip test -fuzz=Fuzz -race -fuzztime=30s


fmt:
	go fmt ./...