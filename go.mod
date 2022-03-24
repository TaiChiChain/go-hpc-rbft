module github.com/ultramesh/flato-rbft

go 1.15

require (
	github.com/gogo/protobuf v1.3.1
	github.com/golang/mock v1.6.0
	github.com/stretchr/testify v1.6.1
	github.com/ultramesh/fancylogger v0.1.3
	github.com/ultramesh/flato-common v0.2.44
	github.com/ultramesh/flato-txpool v0.2.17
)

replace github.com/ultramesh/flato-common => git.hyperchain.cn/ultramesh/flato-common.git v0.2.44

replace github.com/ultramesh/flato-txpool => git.hyperchain.cn/ultramesh/flato-txpool.git v0.2.17

replace github.com/ultramesh/fancylogger => git.hyperchain.cn/ultramesh/fancylogger.git v0.1.3
