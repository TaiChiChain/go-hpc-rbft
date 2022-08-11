PB = $(wildcard *.proto)
MODULE = $(PB:.proto=)

all: $(MODULE)

%: %.proto
		protoc -I=. -I=$(GOPATH)/src  --gogofaster_out=. $<

clean:
		rm -f ./*.pb.go