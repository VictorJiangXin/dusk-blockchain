package protocol

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/dusk-network/dusk-blockchain/pkg/p2p/wire/checksum"
)

type (
	// Gossip is a preprocessor/reader for gossip messages.
	Gossip struct {
		Magic Magic
	}
)

/*
Gossip message/frame

TODO:

*/

// NewGossip returns a gossip preprocessor with the specified magic.
func NewGossip(magic Magic) *Gossip {
	return &Gossip{
		Magic: magic,
	}
}

// Process a message that is passing through, by prepending
// magic and the message checksum, and finally by prepending the length.
func (g *Gossip) Process(m *bytes.Buffer) error {
	cs := checksum.Generate(m.Bytes())
	return WriteFrame(m, g.Magic, cs)
}

// UnpackLength unwraps the incoming packet (likely from a net.Conn struct) and returns the length of the packet without reading the payload (which is left to the user of this method)
func (g *Gossip) UnpackLength(r io.Reader) (uint64, error) {
	packetLength, err := ReadFrame(r)
	if err != nil {
		return 0, err
	}

	magic, err := Extract(r)
	if err != nil {
		return 0, err
	}

	if magic != g.Magic {
		return 0, errors.New("magic mismatch")
	}

	// If packetLength is less than magic.Len(), ln is close to MaxUint64
	// due to integer overflow
	ln := packetLength - uint64(magic.Len())

	if ln > MaxFrameSize {
		return 0, fmt.Errorf("invalid packet length %d", packetLength)
	}

	return ln, nil
}

// ReadMessage reads from the connection
// TODO: Replace ReadMessage with ReadFrame
func (g *Gossip) ReadMessage(src io.Reader) ([]byte, error) {
	length, err := g.UnpackLength(src)
	if err != nil {
		return nil, err
	}

	// read a [length]byte from connection
	buf := make([]byte, int(length))
	_, err = io.ReadFull(src, buf)
	if err != nil {
		return nil, err
	}

	return buf, err
}

// ReadFrame extract message from gossip frame, if no errors found
func (g *Gossip) ReadFrame(src io.Reader) ([]byte, error) {
	length, err := g.UnpackLength(src)
	if err != nil {
		return nil, err
	}

	// read a [length]byte from connection
	buf := make([]byte, int(length))
	_, err = io.ReadFull(src, buf)
	if err != nil {
		return nil, err
	}

	message, cs, err := checksum.Extract(buf)
	if err != nil {
		return nil, fmt.Errorf("extracting checksum: %s", err.Error())
	}

	if !checksum.Verify(message, cs) {
		return nil, fmt.Errorf("invalid checksum: %s", err.Error())
	}

	return message, nil
}
