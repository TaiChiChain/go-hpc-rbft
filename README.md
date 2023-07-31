Axiom-BFT

======

> axiom BFT consensus based on rbft .

## Table of Contents

- [Table of Contents](#table-of-contents)
- [Usage](#usage)
- [API](#api)
  - [Node](#node)
- [Mockgen](#mockgen)
- [GitCZ](#gitcz)
- [Contribute](#contribute)
- [License](#license)

## Usage
axiom-bft provides BFT service based on rbft.

You can initialize a RBFT service by: 
```
Node, err := NewNode(conf)
```
Here, conf is a important structure which contains the config information of RBFT service.

Then you can interact with RBFT core using the methods as below: 

To start the RBFT service by `_ := Node.Start()`.

To stop the RBFT service by `Node.Stop()`.

To propose requests to RBFT core by `_ := Node.Propose(requests)`.

To get the current node status of the RBFT state machine by `ns := Node.Status()`.

To receive and produce consensus messages, you can use `Node.Step(msg)`, which `msg` is the consensus message you will deal with.

To propose the changes of config, you can use `_ := Node.ProposeConfChange(cc)`. Here, `cc` contains the information of the changes of config.
Then application needs to use `Node.ApplyConfChange(cc)` to apply these changes to the local node.

In addition, `Node.ReportExecuted(state)` is invoked after application service has actually applied a batch.
`Node.ReportStateUpdated(State)` is invoked after application service finished stateUpdate which must be triggered by RBFT core before.

Note: To use RBFT core, one should implement several service except txPool. Here list the service that need to be provided:
1. Storage service: to store and restore consensus log.
2. Network service: to send p2p messages between nodes.
3. Crypto service: to access the sign/verify methods from the crypto package.
4. ServiceOutbound service: an application service invoked by RBFT library.

For more information about the service above, you can catch them from file `external.go`.

## API
### Node
import (
    pb "github.com/axiomesh/axiom-bft/rbftpb"
    "github.com/axiomesh/axiom-bft/types"
)

Instantiate Node
```func NewNode(conf Config) (Node, error)```

Start Node Instance
```func (n *node) Start() error```

Stop Node Instance
```func (n *node) Stop()```

Propose Requests
```Propose(requests *consensus.RequestSet) error```

Propose Config Change
```ProposeConfChange(cc *types.ConfChange) error```

Receive and Process Consensus Messages
```Step(msg *consensus.ConsensusMessage)```

RBFT Core Apply Config Change
```ApplyConfChange(cc *types.ConfState)```

Get Current RBFT Core State
```Status() NodeStatus```

Report Block Executed State to RBFT Core
```ReportExecuted(state *types.ServiceState)```

Report State Updated to RBFT Core
```ReportStateUpdated(state *types.ServiceState)```

Report Config Checkpoint Process Finished to RBFT Core
```ReportStableCheckpointFinished(height uint64)```

## Mockgen

Install **mockgen** : `go get github.com/golang/mock/mockgen`

Eg.`mockgen -destination mock/mock_common.go -package common -source consensus.go`

## GitCZ

**Note**: Please use command `npm install` if you are the first time to use `git cz` in this repo.

## Contribute

PRs are welcome!

Small note: If editing the Readme, please conform to the [standard-readme](https://github.com/RichardLitt/standard-readme) specification.

## License

Copyright © 2016-2019 Hangzhou Qulian Technology Co., Ltd.