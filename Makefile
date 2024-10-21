BINARY_NAME = nostr-reader-server.exe
GOFLAGS = -ldflags="-s -w"

all: clean build

build:
	go build $(GOFLAGS) -o ${BINARY_NAME} .
 
run:
	go build $(GOFLAGS) -o ${BINARY_NAME} .
	./${BINARY_NAME}
 
clean:
	rm ${BINARY_NAME}