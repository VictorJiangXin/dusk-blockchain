package peer

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/dusk-network/dusk-blockchain/pkg/config"
	"github.com/dusk-network/dusk-blockchain/pkg/core/consensus/capi"

	log "github.com/sirupsen/logrus"

	"github.com/dusk-network/dusk-blockchain/pkg/p2p/wire/checksum"
	"github.com/dusk-network/dusk-blockchain/pkg/p2p/wire/protocol"
	"github.com/dusk-network/dusk-blockchain/pkg/p2p/wire/topics"
	"github.com/dusk-network/dusk-blockchain/pkg/util/nativeutils/eventbus"
)

var l = log.WithField("process", "peer")

// Connection holds the TCP connection to another node, and it's known protocol magic.
// The `net.Conn` is guarded by a mutex, to allow both multicast and one-to-one
// communication between peers.
type Connection struct {
	lock sync.Mutex
	net.Conn
	gossip *protocol.Gossip
}

// GossipConnector calls Gossip.Process on the message stream incoming from the
// ringbuffer
// It absolves the function previously carried over by the Gossip preprocessor.
type GossipConnector struct {
	gossip *protocol.Gossip
	*Connection
}

func (g *GossipConnector) Write(b []byte) (int, error) {
	buf := bytes.NewBuffer(b)
	if err := g.gossip.Process(buf); err != nil {
		return 0, err
	}

	return g.Connection.Write(buf.Bytes())
}

// Writer abstracts all of the logic and fields needed to write messages to
// other network nodes.
type Writer struct {
	*Connection
	subscriber eventbus.Subscriber
	gossipID   uint32
	keepAlive  time.Duration
	// TODO: add service flag
}

// Reader abstracts all of the logic and fields needed to receive messages from
// other network nodes.
type Reader struct {
	*Connection
	processor    *MessageProcessor
	responseChan chan<- bytes.Buffer
	// TODO: add service flag
}

// NewWriter returns a Writer. It will still need to be initialized by
// subscribing to the gossip topic with a stream handler, and by running the WriteLoop
// in a goroutine..
func NewWriter(conn net.Conn, gossip *protocol.Gossip, subscriber eventbus.Subscriber, keepAlive ...time.Duration) *Writer {
	kas := 30 * time.Second
	if len(keepAlive) > 0 {
		kas = keepAlive[0]
	}
	pw := &Writer{
		Connection: &Connection{
			Conn:   conn,
			gossip: gossip,
		},
		subscriber: subscriber,
		keepAlive:  kas,
	}

	return pw
}

// ReadMessage reads from the connection
func (c *Connection) ReadMessage() ([]byte, error) {
	length, err := c.gossip.UnpackLength(c.Conn)
	if err != nil {
		return nil, err
	}

	// read a [length]byte from connection
	buf := make([]byte, int(length))
	_, err = io.ReadFull(c.Conn, buf)
	if err != nil {
		return nil, err
	}

	return buf, err
}

// Connect will perform the protocol handshake with the peer. If successful
func (w *Writer) Connect() error {
	if err := w.Handshake(); err != nil {
		_ = w.Conn.Close()
		return err
	}

	if config.Get().API.Enabled {
		go func() {
			store := capi.GetStormDBInstance()
			addr := w.Addr()
			peerJSON := capi.PeerJSON{
				Address:  addr,
				Type:     "Writer",
				Method:   "Connect",
				LastSeen: time.Now(),
			}
			err := store.Save(&peerJSON)
			if err != nil {
				log.Error("failed to save peerJSON into StormDB")
			}

			// save count
			peerCount := capi.PeerCount{
				ID:       addr,
				LastSeen: time.Now(),
			}
			err = store.Save(&peerCount)
			if err != nil {
				log.Error("failed to save peerCount into StormDB")
			}
		}()
	}

	return nil
}

// Accept will perform the protocol handshake with the peer.
func (p *Reader) Accept() error {
	if err := p.Handshake(); err != nil {
		_ = p.Conn.Close()
		return err
	}

	if config.Get().API.Enabled {
		go func() {
			store := capi.GetStormDBInstance()
			addr := p.Addr()
			peerJSON := capi.PeerJSON{
				Address:  addr,
				Type:     "Reader",
				Method:   "Accept",
				LastSeen: time.Now(),
			}
			err := store.Save(&peerJSON)
			if err != nil {
				log.Error("failed to save peer into StormDB")
			}

			// save count
			peerCount := capi.PeerCount{
				ID:       addr,
				LastSeen: time.Now(),
			}
			err = store.Save(&peerCount)
			if err != nil {
				log.Error("failed to save peerCount into StormDB")
			}

		}()
	}

	return nil
}

// Create two-way communication with a peer. This function will allow both
// goroutines to run as long as no errors are encountered. Once the first error
// comes through, the context is canceled, and both goroutines are cleaned up.
func Create(ctx context.Context, reader *Reader, writer *Writer, writeQueueChan <-chan bytes.Buffer) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errChan := make(chan error, 1)

	go reader.ReadLoop(ctx, errChan)
	go writer.Serve(ctx, writeQueueChan, errChan)

	// Errors are already getting logged with the proper fields attached.
	// So, we will just wait for one error to come through, after which we can
	// cancel the context and exit this function.
	<-errChan
	close(errChan)
}

// This attempts a non-blocking send to the errChan, which prevents panics
// from sending to a closed channel.
func sendError(errChan chan error, err error) {
	select {
	case errChan <- err:
	default:
	}
}

// Serve utilizes two different methods for writing to the open connection
func (w *Writer) Serve(ctx context.Context, writeQueueChan <-chan bytes.Buffer, errChan chan error) {
	// Any gossip topics are written into interrupt-driven ringBuffer
	// Single-consumer pushes messages to the socket
	g := &GossipConnector{w.gossip, w.Connection}
	w.gossipID = w.subscriber.Subscribe(topics.Gossip, eventbus.NewStreamListener(g))

	// writeQueue - FIFO queue
	// writeLoop pushes first-in message to the socket
	w.writeLoop(ctx, writeQueueChan, errChan)
	w.onDisconnect()
}

func (w *Writer) onDisconnect() {
	log.WithField("address", w.Connection.RemoteAddr().String()).Infof("Connection terminated")
	_ = w.Conn.Close()
	w.subscriber.Unsubscribe(topics.Gossip, w.gossipID)

	if config.Get().API.Enabled {
		go func() {
			store := capi.GetStormDBInstance()
			addr := w.Addr()
			peerJSON := capi.PeerJSON{
				Address:  addr,
				Type:     "Writer",
				Method:   "onDisconnect",
				LastSeen: time.Now(),
			}
			err := store.Save(&peerJSON)
			if err != nil {
				log.Error("failed to save peer into StormDB")
			}

			// delete count
			peerCount := capi.PeerCount{
				ID: addr,
			}
			err = store.Delete(&peerCount)
			if err != nil {
				log.Error("failed to Delete peerCount into StormDB")
			}
		}()
	}
}

func (w *Writer) writeLoop(ctx context.Context, writeQueueChan <-chan bytes.Buffer, errChan chan error) {
	defer func() {
		if config.Get().API.Enabled {
			go func() {
				addr := w.Addr()
				peerCount := capi.PeerCount{
					ID: addr,
				}
				store := capi.GetStormDBInstance()
				// delete count
				err := store.Delete(&peerCount)
				if err != nil {
					log.WithField("process", "writeloop").Error("failed to Delete peerCount into StormDB")
				}
			}()
		}
	}()

	for {
		select {
		case buf := <-writeQueueChan:
			if err := w.gossip.Process(&buf); err != nil {
				l.WithError(err).Warnln("error processing outgoing message")
				continue
			}

			if _, err := w.Connection.Write(buf.Bytes()); err != nil {
				l.WithField("process", "writeloop").WithError(err).Warnln("error writing message")
				sendError(errChan, err)
				return
			}
		case <-ctx.Done():
			log.WithField("process", "writeloop").Debug("context canceled")
			return
		}
	}
}

// ReadLoop will block on the read until a message is read, or until the deadline
// is reached. Should be called in a go-routine, after a successful handshake with
// a peer. Eventual duplicated messages are silently discarded.
func (p *Reader) ReadLoop(ctx context.Context, errChan chan error) {
	// As the peer ReadLoop is at the front-line of P2P network, receiving a
	// malformed frame by an adversary node could lead to a panic.
	// In such situation, the node should survive but adversary conn gets dropped
	// defer func() {
	// 	if r := recover(); r != nil {
	// 		log.Errorf("Peer %s failed with critical issue: %v", p.RemoteAddr(), r)
	// 	}
	// }()

	p.readLoop(ctx, errChan)
}

func (p *Reader) readLoop(ctx context.Context, errChan chan error) {
	defer func() {
		_ = p.Conn.Close()
	}()

	readWriteTimeout := time.Duration(config.Get().Timeout.TimeoutReadWrite) * time.Second // Max idle time for a peer

	// Set up a timer, which triggers the sending of a `keepalive` message
	// when fired.
	keepAliveTime := time.Duration(config.Get().Timeout.TimeoutKeepAliveTime) * time.Second // Send keepalive message after inactivity for this amount of time

	timer := time.NewTimer(keepAliveTime)
	go p.keepAliveLoop(ctx, timer)

	for {
		// Check if context was canceled
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Refresh the read deadline
		err := p.Conn.SetReadDeadline(time.Now().Add(readWriteTimeout))
		if err != nil {
			l.WithError(err).Warnf("error setting read timeout")
			sendError(errChan, err)
			return
		}

		b, err := p.gossip.ReadMessage(p.Conn)
		if err != nil {
			l.WithError(err).Warnln("error reading message")
			sendError(errChan, err)
			return
		}

		message, cs, err := checksum.Extract(b)
		if err != nil {
			l.WithError(err).Warnln("error reading Extract message")
			sendError(errChan, err)
			return
		}

		if !checksum.Verify(message, cs) {
			l.WithError(errors.New("invalid checksum")).Warnln("error reading message")
			sendError(errChan, err)
			return
		}

		go func() {
			// TODO: error here should be checked in order to decrease reputation
			// or blacklist spammers
			startTime := time.Now().UnixNano()
			if err = p.processor.Collect(message, p.responseChan); err != nil {
				l.WithField("process", "readloop").
					WithError(err).Error("failed to process message")
			}

			duration := float64(time.Now().UnixNano()-startTime) / 1000000
			l.WithField("cs", hex.EncodeToString(cs)).
				WithField("len", len(message)).
				WithField("ms", duration).Debug("trace message routing")
		}()

		// Reset the keepalive timer
		timer.Reset(keepAliveTime)

		if config.Get().API.Enabled {
			go func() {
				store := capi.GetStormDBInstance()
				addr := p.Addr()
				// save count
				peerCount := capi.PeerCount{
					ID:       addr,
					LastSeen: time.Now(),
				}
				err = store.Save(&peerCount)
				if err != nil {
					log.Error("failed to save peerCount into StormDB")
				}
			}()
		}
	}
}

func (p *Reader) keepAliveLoop(ctx context.Context, timer *time.Timer) {
	for {
		select {
		case <-timer.C:
			// TODO: why was the error never checked ?
			err := p.Connection.keepAlive()
			if err != nil {
				log.WithError(err).WithField("process", "keepaliveloop").Error("got error back from keepAlive")
				if config.Get().API.Enabled {
					go func() {
						addr := p.Addr()
						peerCount := capi.PeerCount{
							ID: addr,
						}
						store := capi.GetStormDBInstance()
						// delete count
						err = store.Delete(&peerCount)
						if err != nil {
							log.WithField("process", "keepaliveloop").Error("failed to Delete peerCount into StormDB")
						}
					}()
				}

			}

		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}

func (c *Connection) keepAlive() error {
	buf := new(bytes.Buffer)
	if err := topics.Prepend(buf, topics.Ping); err != nil {
		return err
	}

	if err := c.gossip.Process(buf); err != nil {
		return err
	}

	_, err := c.Write(buf.Bytes())
	return err
}

// Write a message to the connection.
// Conn needs to be locked, as this function can be called both by the WriteLoop,
// and by the writer on the ring buffer.
func (c *Connection) Write(b []byte) (int, error) {
	readWriteTimeout := time.Duration(config.Get().Timeout.TimeoutReadWrite) * time.Second // Max idle time for a peer

	c.lock.Lock()
	_ = c.Conn.SetWriteDeadline(time.Now().Add(readWriteTimeout))
	n, err := c.Conn.Write(b)
	c.lock.Unlock()
	return n, err
}

// Addr returns the peer's address as a string.
func (c *Connection) Addr() string {
	return c.Conn.RemoteAddr().String()
}
