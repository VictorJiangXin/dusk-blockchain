package reduction

import (
	"bytes"
	"encoding/hex"
	"time"

	log "github.com/sirupsen/logrus"
	"gitlab.dusk.network/dusk-core/dusk-go/pkg/p2p/wire"
)

type eventStopWatch struct {
	collectedVotesChan chan []wire.Event
	stopChan           chan interface{}
	timer              *timer
}

func newEventStopWatch(collectedVotesChan chan []wire.Event, timer *timer) *eventStopWatch {
	return &eventStopWatch{
		collectedVotesChan: collectedVotesChan,
		stopChan:           make(chan interface{}, 1),
		timer:              timer,
	}
}

func (esw *eventStopWatch) fetch() []wire.Event {
	timer := time.NewTimer(esw.timer.timeout)
	select {
	case <-timer.C:
		esw.timer.timeoutChan <- true
		stop(timer)
		return nil
	case collectedVotes := <-esw.collectedVotesChan:
		stop(timer)
		return collectedVotes
	case <-esw.stopChan:
		stop(timer)
		esw.collectedVotesChan = nil
		return nil
	}
}

func stop(t *time.Timer) {
	if t != nil {
		t.Stop()
		t = nil
	}
}

func (esw *eventStopWatch) stop() {
	esw.stopChan <- true
}

type reducer struct {
	firstStep  *eventStopWatch
	secondStep *eventStopWatch
	ctx        *context

	// TODO: review after demo. used to restart a phase after reduction
	regenerationChannel chan bool
}

func newCoordinator(collectedVotesChan chan []wire.Event, ctx *context,
	regenerationChannel chan bool) *reducer {

	return &reducer{
		firstStep:           newEventStopWatch(collectedVotesChan, ctx.timer),
		secondStep:          newEventStopWatch(collectedVotesChan, ctx.timer),
		ctx:                 ctx,
		regenerationChannel: regenerationChannel,
	}
}

func (c *reducer) begin() {
	log.WithField("process", "reducer").Traceln("Beginning Reduction")
	// this is a blocking call
	events := c.firstStep.fetch()
	log.WithField("process", "reducer").Traceln("First step completed")
	c.ctx.state.IncrementStep()
	hash1 := c.encodeEv(events)
	reductionVote, err := c.ctx.handler.MarshalHeader(hash1, c.ctx.state)
	if err != nil {
		panic(err)
	}
	c.ctx.reductionVoteChan <- reductionVote
	eventsSecondStep := c.secondStep.fetch()
	log.WithField("process", "reducer").Traceln("Second step completed")
	hash2 := c.encodeEv(eventsSecondStep)

	allEvents := append(events, eventsSecondStep...)

	if c.isReductionSuccessful(hash1, hash2, allEvents) {

		log.WithFields(log.Fields{
			"process":    "reducer",
			"votes":      len(allEvents),
			"block hash": hex.EncodeToString(hash1.Bytes()),
		}).Debugln("Reduction successful")
		if err := c.ctx.handler.MarshalVoteSet(hash2, allEvents); err != nil {
			panic(err)
		}
		agreementVote, err := c.ctx.handler.MarshalHeader(hash2, c.ctx.state)
		if err != nil {
			panic(err)
		}

		c.ctx.agreementVoteChan <- agreementVote
	}
	c.ctx.state.IncrementStep()

	c.regenerationChannel <- true
}

func (c *reducer) encodeEv(events []wire.Event) *bytes.Buffer {
	if events == nil {
		events = []wire.Event{nil}
	}
	hash := new(bytes.Buffer)
	if err := c.ctx.handler.EmbedVoteHash(events[0], hash); err != nil {
		panic(err)
	}
	return hash
}

func (c *reducer) isReductionSuccessful(hash1, hash2 *bytes.Buffer, events []wire.Event) bool {
	bothNotNil := hash1 != nil && hash2 != nil
	identicalResults := bytes.Equal(hash1.Bytes(), hash2.Bytes())
	voteSetCorrectLength := len(events) >= c.ctx.committee.Quorum()*2

	return bothNotNil && identicalResults && voteSetCorrectLength
}

func (c *reducer) end() {
	// NOTE: we replace the shared context pointer here, because the `begin`
	// function will adjust the context state, resulting in issues during round
	// updates. should possibly explore another way to adjust step state on the
	// collector
	c.ctx = newCtx(c.ctx.handler, c.ctx.committee, c.ctx.timer.timeout)
	// NOTE: cut off the regeneration channel so that the `begin` call does
	// not mess with the selector, when it triggers regeneration.
	c.regenerationChannel = make(chan bool, 1)
	c.firstStep.stop()
	c.secondStep.stop()
}
