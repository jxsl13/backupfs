

test:
	go test -race -timeout 30s ./...

coverage:
	go test -timeout 30s -race -coverprofile=./coverage ./...
	rm ./coverage

fuzz:
	gotip test -fuzz=Fuzz -race -fuzztime=30s


fmt:
	go fmt ./...