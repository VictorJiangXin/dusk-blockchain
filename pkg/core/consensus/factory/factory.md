# ConsensusFactory

`ConsensusFactory` is responsible for initializing the consensus processes with the proper parameters. It subscribes to the initialization topic and, upon reception of a message, will start all of the components related to consensus. It should also contain all the relevant information for the processes it intends to start up.

## API

* `New(eventBus, rpcBus, timeOut, keys, d, k)` - creates a `ConsensusFactory` by accepting an `EventBus`, an `RPCBus`, and the `timerLength` being the duration of all the phases. It also initializes the channel for listening to the initial _block height_ necessary to begin the consensus.
* `StartConsensus()` - after receiving an initialization message with the Block Height, proceed to start the consensus components by invoking:
  * `reputation.Launch`
  * `generation.Launch`
  * `selection.Launch`
  * `reduction.Launch`
  * `agreement.Launch`

