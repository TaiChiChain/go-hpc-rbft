module github.com/ultramesh/flato-rbft

go 1.12

require (
	github.com/gogo/protobuf v1.3.1
	github.com/golang/mock v1.4.4
	github.com/stretchr/testify v1.6.1
	github.com/ultramesh/fancylogger v0.1.2
	github.com/ultramesh/flato-common v0.2.4
	github.com/ultramesh/flato-txpool v0.2.11
)

replace github.com/ultramesh/crypto-standard => git.hyperchain.cn/ultramesh/crypto-standard.git v0.1.13

replace github.com/ultramesh/flato-common => git.hyperchain.cn/ultramesh/flato-common.git v0.2.4

replace github.com/ultramesh/flato-txpool => git.hyperchain.cn/ultramesh/flato-txpool.git v0.2.11-1

replace github.com/ultramesh/fancylogger => git.hyperchain.cn/ultramesh/fancylogger.git v0.1.2
