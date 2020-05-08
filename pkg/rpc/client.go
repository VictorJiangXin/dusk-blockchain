package rpc

import (
	"context"
	"time"

	"github.com/dusk-network/dusk-protobuf/autogen/go/node"

	"github.com/dusk-network/dusk-blockchain/pkg/util/nativeutils/rpcbus"
	"github.com/dusk-network/dusk-protobuf/autogen/go/rusk"
	logger "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var log = logger.WithFields(logger.Fields{"prefix": "grpc"})

// Client is a wrapper for a gRPC client. It establishes connection with
// the server on startup, and then handles requests from other components
// over the RPCBus.
type Client struct {
	rusk.RuskClient
	node.WalletClient
	node.TransactorClient
	conn           *grpc.ClientConn
	validateSTChan chan rpcbus.Request
	executeSTChan  chan rpcbus.Request
}

// InitRPCClients opens the connection with the Rusk gRPC server, and
// initializes the different clients which can speak to the Rusk server.
//
// As the Rusk server is a fundamental part of the node functionality,
// this function will panic if the connection can not be established
// successfully.
func InitRPCClients(ctx context.Context, address string) *Client {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Panic(err)
	}

	return &Client{
		RuskClient:       rusk.NewRuskClient(conn),
		WalletClient:     node.NewWalletClient(conn),
		TransactorClient: node.NewTransactorClient(conn),
		conn:             conn,
	}
}

// Close the connection to the gRPC server.
func (c *Client) Close() error {
	return c.conn.Close()
}
