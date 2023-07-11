//go:generate pwd

//go:generate mockgen -destination mock_txpool.go -package txpoolmock -source ../tx_pool.go

package txpoolmock
