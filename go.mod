module github.com/ultramesh/flato-rbft

go 1.15

require (
	github.com/gogo/protobuf v1.3.1
	github.com/golang/mock v1.4.4
	github.com/stretchr/testify v1.6.1
	github.com/ultramesh/fancylogger v0.1.2
	github.com/ultramesh/flato-common v0.2.10
	github.com/ultramesh/flato-txpool v0.2.13
)

replace github.com/ultramesh/crypto-standard => git.hyperchain.cn/ultramesh/crypto-standard.git v0.2.3

replace github.com/ultramesh/flato-common => git.hyperchain.cn/ultramesh/flato-common.git v0.2.28-0.20211120015754-1d8986fae40c

replace github.com/ultramesh/flato-txpool => git.hyperchain.cn/ultramesh/flato-txpool.git v0.2.13

replace github.com/ultramesh/fancylogger => git.hyperchain.cn/ultramesh/fancylogger.git v0.1.2
