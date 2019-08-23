module github.com/ultramesh/flato-rbft

go 1.12

require (
	github.com/gogo/protobuf v1.2.1
	github.com/golang/mock v1.3.1
	github.com/ultramesh/flato-common v0.1.0 // indirect
	github.com/ultramesh/flato-event v0.1.0
	github.com/ultramesh/flato-txpool v0.1.0
)

replace github.com/ultramesh/flato-event v0.1.0 => git.hyperchain.cn/ultramesh/flato-event.git v0.1.0

replace github.com/ultramesh/flato-txpool v0.1.0 => git.hyperchain.cn/ultramesh/flato-txpool.git v0.1.0

replace github.com/ultramesh/flato-common v0.1.0 => git.hyperchain.cn/ultramesh/flato-common.git v0.1.0
