package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gocql/gocql"
	scyllacdc "github.com/scylladb/scylla-cdc-go"
)

// Make sure you create the following table before you run this example:
// CREATE TABLE ks.tbl (pk int, ck int, v int, PRIMARY KEY (pk, ck)) WITH cdc = {'enabled': 'true'};

func main() {
	cluster := gocql.NewCluster("127.0.0.1")
	cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())
	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatal(err)
		return
	}
	defer session.Close()

	logger := log.New(os.Stderr, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)

	cfg := &scyllacdc.ReaderConfig{
		Session:               session,
		TableNames:            []string{"ks.tbl"},
		ChangeConsumerFactory: changeConsumerFactory,
		Logger:                logger,
		Advanced: scyllacdc.AdvancedReaderConfig{
			ChangeAgeLimit:       15 * time.Minute,
			ConfidenceWindowSize: 10 * time.Second,
			PostQueryDelay:       5 * time.Second,
			PostFailedQueryDelay: 5 * time.Second,
			QueryTimeWindowSize:  5 * 60 * time.Second,
		},
	}

	reader, err := scyllacdc.NewReader(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}

	if err := reader.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func consumeChange(ctx context.Context, tableName string, c scyllacdc.Change) error {
	for _, changeRow := range c.Delta {
		pkRaw, _ := changeRow.GetValue("pk")
		ckRaw, _ := changeRow.GetValue("ck")
		v := changeRow.GetAtomicChange("v")

		pk := pkRaw.(*int)
		ck := ckRaw.(*int)

		fmt.Printf("[%s] Operation: %s, pk: %s, ck: %s\n", tableName, changeRow.GetOperation(),
			nullableIntToStr(pk), nullableIntToStr(ck))

		if v.IsDeleted {
			fmt.Printf("  Column v was set to null/deleted\n")
		} else {
			vInt := v.Value.(*int)
			if vInt != nil {
				fmt.Printf("  Column v was set to %d\n", *vInt)
			} else {
				fmt.Print("  Column v was not changed\n")
			}
		}
	}

	return nil
}

func nullableIntToStr(i *int) string {
	if i == nil {
		return "null"
	}
	return fmt.Sprintf("%d", *i)
}

var changeConsumerFactory = scyllacdc.MakeChangeConsumerFactoryFromFunc(consumeChange)
