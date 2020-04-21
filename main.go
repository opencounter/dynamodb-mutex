package main

import (
	"cirello.io/dynamolock"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/coreos/go-systemd/v22/daemon"
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	logger := log.New(os.Stdout, "dynamodb-mutex: ", log.Lshortfile)
	awsSession := session.Must(session.NewSession())
	tableName, found := os.LookupEnv("DYNAMODB_TABLE_NAME")
	if !found {
		logger.Fatal("Must provide DYNAMODB_TABLE_NAME")
	}
	if len(os.Args) < 2 {
		logger.Fatalf("Usage %s KEY OWNER_NAME", os.Args[0])
	}
	key := os.Args[1]
	var ownerName string
	if len(os.Args) >= 3 {
		ownerName = os.Args[2]
	} else {
		logger.Printf("Fetching EC2 instance ID via instance metadata API...")
		ec2 := ec2metadata.New(awsSession)
		idDoc, err := ec2.GetInstanceIdentityDocument()
		if err != nil {
			logger.Fatalf("No OwnerName given and failed to infer via EC2 metadata: %s", err)
		}
		ownerName = idDoc.InstanceID
	}

	svc := dynamodb.New(awsSession)
	c, err := dynamolock.New(svc,
		tableName,
		dynamolock.WithOwnerName(ownerName),
		dynamolock.WithLogger(logger),
	)
	if err != nil {
		logger.Fatal(err)
	}
	defer c.Close()

	logger.Printf("Attempting lock via DDB-table=%s key=%s ownerName=%s...\n", tableName, key, ownerName)
	lock, err := c.AcquireLock(key, dynamolock.WithRefreshPeriod(10*time.Second), dynamolock.WithAdditionalTimeToWaitForLock(10*time.Minute)) //, dynamolock.FailIfLocked())
	if err != nil {
		logger.Fatal(err)
	}
	logger.Printf("acquired lock on table %s, key %s by %s", tableName, key, ownerName)

	ok, err := daemon.SdNotify(false, daemon.SdNotifyReady)
	if err != nil {
		logger.Printf("sd_notify: %s", err)
	} else if !ok {
		logger.Println("sd_notify: not supported")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	logger.Println("got SIGINT, stopping")

	// FIXME: This always seems to fail: `sd_notify: dial unixgram /run/systemd/notify: connect: connection refused`
	ok, err = daemon.SdNotify(false, daemon.SdNotifyStopping)
	if err != nil {
		logger.Printf("sd_notify: %s", err)
	} else if !ok {
		logger.Println("sd_notify: not supported")
	}

	logger.Println("cleaning lock")
	success, err := c.ReleaseLock(lock, dynamolock.WithDeleteLock(false))
	if !success {
		logger.Fatal("lost lock before release")
	}
	if err != nil {
		logger.Fatal("error releasing lock:", err)
	}
}
