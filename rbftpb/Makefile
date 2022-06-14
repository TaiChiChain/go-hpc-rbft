PB = $(wildcard *.proto)
MODULE = $(PB:.proto=)

all: $(MODULE)

%: %.proto
		protoc -I=. -I=$(GOPATH)/src -I=$(GOPATH)/src/github.com/hyperchain/go-hpc-common/types/protos  --gogofaster_out=. $<

clean:
		rm -f ./*.pb.go