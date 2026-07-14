# Builds the device-side C client library and its tests.
#
# Unit tests (test_sha256, test_protocol) are pure and need nothing else
# running: `make test`.
#
# test_integration needs a live server to talk to; it is not run by `make
# test` because managing that server's lifecycle isn't this Makefile's job.
# Start one (e.g. `go run ./cmd/server` from the repo root, or set
# UBIBOT_LISTEN_ADDR=:PORT first) and then run:
#   make build/test_integration
#   build/test_integration 127.0.0.1 8080

CC := gcc
CFLAGS := -std=c11 -Wall -Wextra -Iinclude -Itest
LDFLAGS :=

ifeq ($(OS),Windows_NT)
LDFLAGS += -lws2_32
EXE := .exe
else
EXE :=
endif

BUILD := build

CODEC_SRC := src/ub_sha256.c src/ub_hmac_sha256.c src/ub_json.c src/ub_protocol.c
TRANSPORT_SRC := src/ub_transport_posix.c

.PHONY: all test clean

all: test

test: $(BUILD)/test_sha256$(EXE) $(BUILD)/test_protocol$(EXE)
	$(BUILD)/test_sha256$(EXE)
	$(BUILD)/test_protocol$(EXE)

$(BUILD)/test_sha256$(EXE): test/test_sha256.c $(CODEC_SRC) | $(BUILD)
	$(CC) $(CFLAGS) -o $@ test/test_sha256.c $(CODEC_SRC)

$(BUILD)/test_protocol$(EXE): test/test_protocol.c $(CODEC_SRC) | $(BUILD)
	$(CC) $(CFLAGS) -o $@ test/test_protocol.c $(CODEC_SRC)

$(BUILD)/test_integration$(EXE): test/test_integration.c $(CODEC_SRC) $(TRANSPORT_SRC) | $(BUILD)
	$(CC) $(CFLAGS) -o $@ test/test_integration.c $(CODEC_SRC) $(TRANSPORT_SRC) $(LDFLAGS)

$(BUILD):
	mkdir -p $(BUILD)

clean:
	rm -rf $(BUILD)
