//go:generate pwd

//go:generate go install github.com/golang/mock/mockgen@v1.7.0-rc.1
//go:generate mockgen -destination mock_mempool.go -package mempoolmock -source ../mempool.go

package mempoolmock

// NOTICE: need replace all mempool.T to T
