dynamodb-mutex-linux-amd64: *.go go.mod go.sum
	GOOS=linux GOARCH=amd64 go build -o $@

docker: dynamodb-mutex-linux-amd64
	docker build -t opencounter/dynamodb-mutex .
