package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	api "proglog/api/v1"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	addr := flag.String("addr", "34.142.144.9:8400", "service address")
	flag.Parse()
	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}

	{

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

	{
		healthClient := healthpb.NewHealthClient(conn)

		res, err := healthClient.Check(context.Background(), &healthpb.HealthCheckRequest{})
		if err != nil {
			log.Fatal(err)
		}
		if res.Status != healthpb.HealthCheckResponse_SERVING {
			log.Fatal(res.Status)
		}
		fmt.Println("health check oke")
	}
}
