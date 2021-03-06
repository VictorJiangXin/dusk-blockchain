package peer

import (
	"bytes"
	"net"
	"os"
	"testing"
	"time"

	cfg "github.com/dusk-network/dusk-blockchain/pkg/config"
	"github.com/stretchr/testify/require"

	_ "github.com/dusk-network/dusk-blockchain/pkg/core/database/lite"
	"github.com/dusk-network/dusk-blockchain/pkg/p2p/peer/dupemap"
	"github.com/dusk-network/dusk-blockchain/pkg/p2p/wire/protocol"
	"github.com/dusk-network/dusk-blockchain/pkg/util/nativeutils/eventbus"
)

func TestHandshake(t *testing.T) {
	//setup viper timeout
	cwd, err := os.Getwd()
	require.Nil(t, err)

	r, err := cfg.LoadFromFile(cwd + "/../../../dusk.toml")
	require.Nil(t, err)
	cfg.Mock(&r)

	eb := eventbus.New()

	processor := NewMessageProcessor(eb)
	factory := NewReaderFactory(processor)

	client, srv := net.Pipe()

	go func() {
		responseChan := make(chan bytes.Buffer, 100)
		peerReader, err := factory.SpawnReader(srv, protocol.NewGossip(protocol.TestNet), dupemap.NewDupeMap(0), responseChan)
		if err != nil {
			panic(err)
		}

		if err := peerReader.Accept(); err != nil {
			panic(err)
		}
	}()

	time.Sleep(500 * time.Millisecond)
	g := protocol.NewGossip(protocol.TestNet)
	pw := NewWriter(client, g, eb)
	defer func() {
		_ = pw.Conn.Close()
	}()
	if err := pw.Handshake(); err != nil {
		t.Fatal(err)
	}
}
