module github.com/ultramesh/flato-rbft

go 1.17

require (
	github.com/gogo/protobuf v1.3.1
	github.com/golang/mock v1.6.0
	github.com/stretchr/testify v1.6.1
	github.com/ultramesh/fancylogger v0.1.3
	github.com/ultramesh/flato-common v0.2.48
	github.com/ultramesh/flato-txpool v0.2.17
)

require (
	github.com/golang/protobuf v1.5.1 // indirect
	github.com/json-iterator/go v1.1.10 // indirect
	github.com/meshplus/crypto v0.0.9 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/pingcap/failpoint v0.0.0-20191029060244-12f4ac2fd11d // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/willf/bitset v1.1.11 // indirect
	github.com/willf/bloom v2.0.3+incompatible // indirect
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550 // indirect
	golang.org/x/net v0.0.0-20210405180319-a5a99cb37ef4 // indirect
	golang.org/x/text v0.3.3 // indirect
	google.golang.org/genproto v0.0.0-20200526211855-cb27e3aa2013 // indirect
	google.golang.org/grpc v1.38.0 // indirect
	google.golang.org/protobuf v1.26.0 // indirect
)

replace github.com/ultramesh/flato-common => git.hyperchain.cn/ultramesh/flato-common.git v0.2.48

replace github.com/ultramesh/flato-txpool => git.hyperchain.cn/ultramesh/flato-txpool.git v0.2.17

replace github.com/ultramesh/fancylogger => git.hyperchain.cn/ultramesh/fancylogger.git v0.1.3
