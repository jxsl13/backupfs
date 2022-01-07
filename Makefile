

test:
	go test -timeout 30s ./...


fuzz:
	gotip test -fuzz=Fuzz