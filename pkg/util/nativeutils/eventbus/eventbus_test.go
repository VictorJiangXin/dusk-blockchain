package eventbus

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/dusk-network/dusk-blockchain/pkg/p2p/wire/message"
	"github.com/dusk-network/dusk-blockchain/pkg/p2p/wire/topics"
	"github.com/sirupsen/logrus"
	assert "github.com/stretchr/testify/require"
)

//*****************
// EVENTBUS TESTS
//*****************
func TestNewEventBus(t *testing.T) {
	eb := New()
	assert.NotNil(t, eb)
}

//******************
// SUBSCRIBER TESTS
//******************
func TestListenerMap(t *testing.T) {
	lm := newListenerMap()
	_, ss := CreateGossipStreamer()
	listener := NewStreamListener(ss)
	lm.Store(topics.Test, listener)

	listeners := lm.Load(topics.Test)
	assert.Equal(t, 1, len(listeners))
	assert.Equal(t, listener, listeners[0].Listener)
}

func TestSubscribe(t *testing.T) {
	eb := New()
	myChan := make(chan message.Message, 10)
	cl := NewChanListener(myChan)
	assert.NotNil(t, eb.Subscribe(topics.Test, cl))
}

func TestUnsubscribe(t *testing.T) {
	eb, myChan, id := newEB(t)
	eb.Unsubscribe(topics.Test, id)
	msg := message.New(topics.Test, bytes.NewBufferString("whatever2"))
	eb.Publish(topics.Test, msg)

	select {
	case <-myChan:
		assert.FailNow(t, "We should have not received message")
	case <-time.After(50 * time.Millisecond):
		// success
	}
}

//*********************
// STREAMER TESTS
//*********************
func TestStreamer(t *testing.T) {
	topic := topics.Gossip
	bus, streamer := CreateFrameStreamer(topic)
	msg := message.New(topics.Test, bytes.NewBufferString("pluto")) //nolint
	bus.Publish(topic, msg)

	packet, err := streamer.(*SimpleStreamer).Read()
	assert.NoError(t, err)
	// first 4 bytes of packet are the checksum
	assert.Equal(t, "pluto", string(packet[4:]))
}

//******************
// MULTICASTER TESTS
//******************
func TestDefaultListener(t *testing.T) {
	eb := New()
	msgChan := make(chan message.Message)

	eb.AddDefaultTopic(topics.Reject)
	eb.AddDefaultTopic(topics.Unknown)
	eb.SubscribeDefault(NewChanListener(msgChan))

	m := message.New(topics.Reject, *bytes.NewBufferString("pluto")) //nolint
	eb.Publish(topics.Reject, m)
	msg := <-msgChan
	assert.Equal(t, topics.Reject, msg.Category())

	payload := msg.Payload().(message.SafeBuffer)
	assert.Equal(t, []byte("pluto"), (&payload).Bytes())

	m = message.New(topics.Unknown, bytes.NewBufferString("pluto")) //nolint
	eb.Publish(topics.Unknown, m)
	msg = <-msgChan
	assert.Equal(t, topics.Unknown, msg.Category())
	payload = msg.Payload().(message.SafeBuffer)
	assert.Equal(t, []byte("pluto"), (&payload).Bytes())

	m = message.New(topics.Gossip, bytes.NewBufferString("pluto")) //nolint
	eb.Publish(topics.Gossip, m)
	select {
	case <-msgChan:
		t.FailNow()
	case <-time.After(100 * time.Millisecond):
		//all good
	}
}

//****************
// SETUP FUNCTIONS
//****************
//nolint
func newEB(t *testing.T) (*EventBus, chan message.Message, uint32) {
	eb := New()
	myChan := make(chan message.Message, 10)
	cl := NewChanListener(myChan)
	id := eb.Subscribe(topics.Test, cl)
	assert.NotNil(t, id)
	b := bytes.NewBufferString("whatever")
	m := message.New(topics.Test, *b)
	eb.Publish(topics.Test, m)

	select {
	case received := <-myChan:
		payload := received.Payload().(message.SafeBuffer)
		assert.Equal(t, "whatever", (&payload).String())
	case <-time.After(50 * time.Millisecond):
		assert.FailNow(t, "We should have received a message by now")
	}

	return eb, myChan, id
}

// Test that a streaming goroutine is killed when the exit signal is sent
// nolint
func TestExitChan(t *testing.T) {
	// suppressing annoying logging of expected errors
	logrus.SetLevel(logrus.FatalLevel)
	eb := New()
	topic := topics.Test
	sl := NewStreamListener(&mockWriteCloser{})
	_ = eb.Subscribe(topic, sl)

	// Put something on ring buffer
	val := new(bytes.Buffer)
	val.Write([]byte{0})
	m := message.New(topic, *val)
	eb.Publish(topic, m)
	// Wait for event to be handled
	// NB: 'Writer' must return error to force consumer termination
	time.Sleep(100 * time.Millisecond)

	l := eb.listeners.Load(topic)
	for _, listener := range l {
		if streamer, ok := listener.Listener.(*StreamListener); ok {
			assert.True(t, streamer.ringbuffer.Closed())
			return
		}
	}

	assert.FailNow(t, "stream listener not found")
}

type mockWriteCloser struct {
}

func (m *mockWriteCloser) Write(data []byte) (int, error) {
	return 0, errors.New("failed")
}

func (m *mockWriteCloser) Close() error {
	return nil
}
