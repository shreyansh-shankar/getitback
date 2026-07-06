BINARY = getitback
CMD_DIR = ./cmd/getitback
BUILD_FLAGS = -ldflags="-s -w"

.PHONY: all build clean install

all: build

build:
	go build $(BUILD_FLAGS) -o $(BINARY) $(CMD_DIR)

clean:
	rm -f $(BINARY)

install: build
	install -m 755 $(BINARY) /usr/local/bin/$(BINARY)
