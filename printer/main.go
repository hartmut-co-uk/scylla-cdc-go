package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/gocql/gocql"
	scylla_cdc "github.com/piodul/scylla-cdc-go"
)

func main() {
	var (
		keyspace string
		table    string
		source   string
	)

	flag.StringVar(&keyspace, "keyspace", "", "keyspace name")
	flag.StringVar(&table, "table", "", "table name")
	flag.StringVar(&source, "source", "127.0.0.1", "address of a node in the cluster")
	flag.Parse()

	// Configure a session first
	cluster := gocql.NewCluster(source)
	cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())
	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	cfg := scylla_cdc.NewReaderConfig(
		session,
		scylla_cdc.MakeChangeConsumerFactoryFromFunc(printerConsumer),
		&scylla_cdc.NoProgressManager{},
		keyspace+"."+table,
	)
	cfg.Logger = log.New(os.Stderr, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)

	reader, err := scylla_cdc.NewReader(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}

	// React to Ctrl+C signal, and stop gracefully after the first signal
	// Second signal exits the process
	signalC := make(chan os.Signal)
	go func() {
		<-signalC
		reader.Stop()

		<-signalC
		os.Exit(1)
	}()
	signal.Notify(signalC, os.Interrupt)

	if err := reader.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func printerConsumer(ctx context.Context, tableName string, c scylla_cdc.Change) error {
	fmt.Printf("[%s %s]:\n", c.StreamID, c.Time.String())
	if len(c.Preimage) > 0 {
		fmt.Println("  PREIMAGE:")
		for _, r := range c.Preimage {
			fmt.Printf("    %s\n", r)
		}
	}
	if len(c.Delta) > 0 {
		fmt.Println("  DELTA:")
		for _, r := range c.Delta {
			fmt.Printf("    %s\n", r)
		}
	}
	if len(c.Postimage) > 0 {
		fmt.Println("  POSTIMAGE:")
		for _, r := range c.Postimage {
			fmt.Printf("    %s\n", r)
		}
	}
	fmt.Println()

	return nil
}
