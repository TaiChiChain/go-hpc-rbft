all: clean pb

clean:
	rm -f ./*.pb.go

pb:
	protoc -I=. \
	-I${GOPATH}/src \
	--gogofaster_out=:. \
	*.proto
