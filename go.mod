module github.com/hyperchain/go-hpc-rbft

go 1.17

require (
	github.com/gogo/protobuf v1.3.2
	github.com/golang/mock v1.6.0
	github.com/hyperchain/go-hpc-common v0.3.0
	github.com/hyperchain/go-hpc-txpool v0.3.0
	github.com/stretchr/testify v1.6.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/protobuf v1.5.1 // indirect
	github.com/json-iterator/go v1.1.11 // indirect
	github.com/meshplus/crypto v0.0.9 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/pingcap/failpoint v0.0.0-20191029060244-12f4ac2fd11d // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/willf/bitset v1.1.11 // indirect
	github.com/willf/bloom v2.0.3+incompatible // indirect
	golang.org/x/crypto v0.0.0-20220518034528-6f7dac969898 // indirect
	golang.org/x/net v0.0.0-20211209124913-491a49abca63 // indirect
	golang.org/x/sys v0.0.0-20220412211240-33da011f77ad // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20200526211855-cb27e3aa2013 // indirect
	google.golang.org/grpc v1.38.0 // indirect
	google.golang.org/protobuf v1.26.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c // indirect
)

replace github.com/hyperchain/go-hpc-common => git.hyperchain.cn/hyperchain/go-hpc-common.git v0.3.0

replace github.com/hyperchain/go-hpc-txpool => git.hyperchain.cn/hyperchain/go-hpc-txpool.git v0.3.0

replace gopkg.in/yaml.v2 => github.com/go-yaml/yaml v0.0.0-20200121171940-53403b58ad1b

replace github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.11.1
