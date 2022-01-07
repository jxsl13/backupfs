

test:
	go test -timeout 30s ./...

coverage:
	go test -timeout 30s -coverprofile=./coverage ./...
	rm ./coverage

fuzz:
	gotip test -fuzz=Fuzz