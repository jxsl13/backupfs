

test:
	gotip test -timeout 30s -run ^Test.+ github.com/jxsl13/backupfs


fuzz:
	gotip test -fuzz=Fuzz