dynamodb-mutex-amd64: *.go go.mod go.sum
	GOOS=linux GOARCH=amd64 go build -o $@
