FROM alpine

RUN apk add --update ca-certificates

COPY dynamodb-mutex-linux-amd64 /dynamodb-mutex

ENTRYPOINT ["/dynamodb-mutex"]

STOPSIGNAL SIGINT
