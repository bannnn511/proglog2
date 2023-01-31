package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	api "proglog/api/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	addr := flag.String("addr", ":8400", "service address")
	flag.Parse()
	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}

	client := api.NewLogClient(conn)
	res, err := client.GetServers(context.Background(), &api.GetServersRequest{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("servers:")
	for _, server := range res.Servers {
		fmt.Printf("\t- %v\n", server)
	}
}
