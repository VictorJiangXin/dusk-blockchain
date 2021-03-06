package responding

import (
	"bytes"

	"github.com/dusk-network/dusk-blockchain/pkg/core/data/block"
	"github.com/dusk-network/dusk-blockchain/pkg/core/data/ipc/transactions"
	"github.com/dusk-network/dusk-blockchain/pkg/core/database"
	"github.com/dusk-network/dusk-blockchain/pkg/p2p/wire/message"
	"github.com/dusk-network/dusk-blockchain/pkg/p2p/wire/topics"
	"github.com/dusk-network/dusk-blockchain/pkg/util/nativeutils/rpcbus"
)

// DataBroker is a processing unit responsible for handling GetData messages. It
// maintains a connection to the outgoing message queue of the peer it receives this
// message from.
type DataBroker struct {
	db     database.DB
	rpcBus *rpcbus.RPCBus
}

// NewDataBroker returns an initialized DataBroker.
func NewDataBroker(db database.DB, rpcBus *rpcbus.RPCBus) *DataBroker {
	return &DataBroker{
		db:     db,
		rpcBus: rpcBus,
	}
}

// SendItems takes a GetData message from the wire, and iterates through the list,
// sending back each item's complete data to the requesting peer.
func (d *DataBroker) SendItems(m message.Message) ([]bytes.Buffer, error) {
	msg := m.Payload().(message.Inv)

	bufs := make([]bytes.Buffer, 0, len(msg.InvList))
	for _, obj := range msg.InvList {
		switch obj.Type {
		case message.InvTypeBlock:
			// Fetch block from local state. It must be available
			var b *block.Block
			err := d.db.View(func(t database.Transaction) error {
				var err error
				b, err = t.FetchBlock(obj.Hash)
				return err
			})

			if err != nil {
				return nil, err
			}

			// Send the block data back to the initiator node as topics.Block msg
			buf, err := marshalBlock(b)
			if err != nil {
				return nil, err
			}

			bufs = append(bufs, *buf)
		case message.InvTypeMempoolTx:
			// Try to retrieve tx from local mempool state. It might not be
			// available
			txs, err := GetMempoolTxs(d.rpcBus, obj.Hash)
			if err != nil {
				return nil, err
			}

			if len(txs) != 0 {
				// Send topics.Tx with the tx data back to the initiator
				buf, err := marshalTx(txs[0])
				if err != nil {
					return nil, err
				}

				bufs = append(bufs, *buf)
			}

			// A txID will not be found in a few situations:
			//
			// - The node has restarted and lost this Tx
			// - The node has recently accepted a block that includes this Tx
			// No action to run in these cases.
		}

	}

	return bufs, nil
}

func marshalBlock(b *block.Block) (*bytes.Buffer, error) {
	//TODO: following is more efficient, saves an allocation and avoids the explicit Prepend
	// buf := topics.Topics[topics.Block].Buffer
	buf := new(bytes.Buffer)
	if err := message.MarshalBlock(buf, b); err != nil {
		return nil, err
	}

	if err := topics.Prepend(buf, topics.Block); err != nil {
		return nil, err
	}

	return buf, nil
}

func marshalTx(tx transactions.ContractCall) (*bytes.Buffer, error) {
	//TODO: following is more efficient, saves an allocation and avoids the explicit Prepend
	// buf := topics.Topics[topics.Block].Buffer
	buf := new(bytes.Buffer)
	if err := transactions.Marshal(buf, tx); err != nil {
		return nil, err
	}

	if err := topics.Prepend(buf, topics.Tx); err != nil {
		return nil, err
	}

	return buf, nil
}
