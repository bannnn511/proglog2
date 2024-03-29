package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"proglog/internal/agent"
	"proglog/internal/config"
	"syscall"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	cli := &cli{}

	healthCheckCmd := &cobra.Command{
		Use: "health",
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := flag.String("addr", ":8400", "service address")
			flag.Parse()
			conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatal(err)
			}

			client := healthpb.NewHealthClient(conn)
			res, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{})
			if err != nil {
				log.Fatal(err)
			}
			if res.Status != healthpb.HealthCheckResponse_SERVING {
				return errors.New("not serving")
			}
			fmt.Println("health check oke")

			return nil
		},
	}

	cmd := &cobra.Command{
		Use:     "proglog",
		PreRunE: cli.setupConfig,
		RunE:    cli.run,
	}
	cmd.AddCommand(healthCheckCmd)

	if err := setupFlags(cmd); err != nil {
		log.Fatal(err)
	}

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func setupFlags(cmd *cobra.Command) error {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	cmd.Flags().String("config-file", "", "Path to config file.")

	dataDir := path.Join(os.TempDir(), "proglog")
	cmd.Flags().String("data-dir",
		dataDir,
		"Directory to store log and Raft data.")

	cmd.Flags().String("node-name", hostname, "Unique server ID.")
	cmd.Flags().String("bind-addr", "127.0.0.1:8401",
		"Address to bind Serf on.")
	cmd.Flags().Int("rpc-port",
		8400,
		"Port for RPC clients (and Raft) connections.")
	cmd.Flags().StringSlice("start-join-addrs",
		nil,
		"Serf addresses to join.")
	cmd.Flags().Bool("bootstrap", false, "Bootstrap the cluster.")
	cmd.Flags().String("acl-model-file", "", "Path to ACL model.")
	cmd.Flags().String("acl-policy-file", "", "Path to ACL policy.")
	cmd.Flags().String("server-tls-cert-file", "", "Path to server tls cert.")
	cmd.Flags().String("server-tls-key-file", "", "Path to server tls key.")
	cmd.Flags().String("server-tls-ca-file",
		"",
		"Path to server certificate authority.")
	cmd.Flags().String("peer-tls-cert-file", "", "Path to peer tls cert.")
	cmd.Flags().String("peer-tls-key-file", "", "Path to peer tls key.")
	cmd.Flags().String("peer-tls-ca-file",
		"",
		"Path to peer certificate authority.")

	return viper.BindPFlags(cmd.Flags())
}

type cli struct {
	cfg
}

func (c *cli) run(_ *cobra.Command, _ []string) error {
	var err error

	fmt.Println("[Config]", c.cfg.Config)
	a, err := agent.New(c.cfg.Config)
	if err != nil {
		return err
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	<-sigc

	return a.Shutdown()
}

type cfg struct {
	agent.Config
	ServerTLSConfig config.TLSConfig
	PeerTLSConfig   config.TLSConfig
}

func (c *cli) setupConfig(cmd *cobra.Command, args []string) error {
	var err error

	configFile, err := cmd.Flags().GetString("config-file")
	if err != nil {
		return err
	}

	viper.SetConfigFile(configFile)
	if err = viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	c.cfg.DataDir = viper.GetString("data-dir")
	c.cfg.NodeName = viper.GetString("node-name")
	c.cfg.BindAddr = viper.GetString("bind-addr")
	c.cfg.RpcPort = viper.GetInt("rpc-port")
	c.cfg.StartJoinAddresses = viper.GetStringSlice("start-join-addrs")
	c.cfg.Boostrap = viper.GetBool("bootstrap")
	c.cfg.ACLModelFile = viper.GetString("acl-mode-file")
	c.cfg.ACLPolicyFile = viper.GetString("acl-policy-file")
	c.cfg.ServerTLSConfig.CertFile = viper.GetString("server-tls-cert-file")
	c.cfg.ServerTLSConfig.KeyFile = viper.GetString("server-tls-key-cert-file")
	c.cfg.ServerTLSConfig.CAFile = viper.GetString("server-tls-ca-file")
	c.cfg.PeerTLSConfig.CertFile = viper.GetString("peer-tls-cert-file")
	c.cfg.PeerTLSConfig.KeyFile = viper.GetString("peer-tls-key-file")
	c.cfg.PeerTLSConfig.CAFile = viper.GetString("peer-tls-ca-file")

	if c.cfg.ServerTLSConfig.CertFile != "" &&
		c.cfg.ServerTLSConfig.KeyFile != "" {
		c.cfg.ServerTLSConfig.Server = true
		c.cfg.Config.ServerTLSConfig, err = config.SetupTLSConfig(
			c.cfg.ServerTLSConfig,
		)
		if err != nil {
			return err
		}
	}

	if c.cfg.PeerTLSConfig.CertFile != "" &&
		c.cfg.PeerTLSConfig.KeyFile != "" {
		c.cfg.Config.PeerTLSConfig, err = config.SetupTLSConfig(
			c.cfg.PeerTLSConfig,
		)
		if err != nil {
			return err
		}
	}

	return nil
}
