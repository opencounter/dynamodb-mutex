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
	awsSession := session.Must(session.NewSession())
	tableName, found := os.LookupEnv("DYNAMODB_TABLE_NAME")
	if !found {
		log.Fatal("Must provide DYNAMODB_TABLE_NAME")
	}
	if len(os.Args) < 2 {
		log.Fatalf("Usage %s KEY OWNER_NAME", os.Args[0])
	}
	key := os.Args[1]
	var ownerName string
	if len(os.Args) >= 3 {
		ownerName = os.Args[2]
	} else {
		ec2 := ec2metadata.New(awsSession)
		idDoc, err := ec2.GetInstanceIdentityDocument()
		if err != nil {
			log.Fatalf("No OwnerName given and failed to infer via EC2 metadata: %s", err)
		}
		ownerName = idDoc.InstanceID
	}

	svc := dynamodb.New(awsSession)
	c, err := dynamolock.New(svc,
		tableName,
		dynamolock.WithOwnerName(ownerName),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	lock, err := c.AcquireLock(key, dynamolock.WithRefreshPeriod(10*time.Second), dynamolock.WithAdditionalTimeToWaitForLock(10*time.Minute)) //, dynamolock.FailIfLocked())
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("acquired lock on table %s, key %s by %s", tableName, key, ownerName)

	ok, err := daemon.SdNotify(false, daemon.SdNotifyReady)
	if err != nil {
		log.Printf("sd_notify: %s", err)
	} else if !ok {
		log.Println("sd_notify: not supported")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	log.Println("got SIGINT, stopping")
	ok, err = daemon.SdNotify(false, daemon.SdNotifyStopping)
	if err != nil {
		log.Printf("sd_notify: %s", err)
	} else if !ok {
		log.Println("sd_notify: not supported")
	}
	log.Println("cleaning lock")
	success, err := c.ReleaseLock(lock, dynamolock.WithDeleteLock(false))
	if !success {
		log.Fatal("lost lock before release")
	}
	if err != nil {
		log.Fatal("error releasing lock:", err)
	}
}
