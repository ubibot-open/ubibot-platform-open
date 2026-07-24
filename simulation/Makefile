# Lightweight alternative to CMakeLists.txt for a quick native build.
# Works with both native gcc
# (Linux) and a mingw-w64 cross-compiler (set CC=x86_64-w64-mingw32-gcc) or
# native MinGW gcc on Windows.
#
# make            # builds ub_device_sim + runs the pure unit tests
# make sim        # just the simulator binary
# make test       # just the unit tests (test_protocol)
# make build/test_integration$(EXE)   # needs a live server, see test/test_integration.c

CC := gcc
CFLAGS := -std=c11 -Wall -Wextra -Icore/include -Itransport/include -Iapp/include -Itest
LDFLAGS :=

ifeq ($(OS),Windows_NT)
LDFLAGS += -lws2_32
EXE := .exe
else
EXE :=
endif

BUILD := build

CORE_SRC := core/src/ub_json.c core/src/ub_protocol.c
TRANSPORT_SRC := transport/src/ub_transport_sockets.c
APP_SRC := app/src/ub_platform_host.c app/src/ub_sensors_sim.c app/src/ub_device.c

.PHONY: all sim test clean

all: sim test

sim: $(BUILD)/ub_device_sim$(EXE)

test: $(BUILD)/test_protocol$(EXE)
	$(BUILD)/test_protocol$(EXE)

$(BUILD)/ub_device_sim$(EXE): app/src/main.c $(CORE_SRC) $(TRANSPORT_SRC) $(APP_SRC) | $(BUILD)
	$(CC) $(CFLAGS) -o $@ app/src/main.c $(CORE_SRC) $(TRANSPORT_SRC) $(APP_SRC) $(LDFLAGS)

$(BUILD)/test_protocol$(EXE): test/test_protocol.c $(CORE_SRC) | $(BUILD)
	$(CC) $(CFLAGS) -o $@ test/test_protocol.c $(CORE_SRC)

$(BUILD)/test_integration$(EXE): test/test_integration.c $(CORE_SRC) $(TRANSPORT_SRC) | $(BUILD)
	$(CC) $(CFLAGS) -o $@ test/test_integration.c $(CORE_SRC) $(TRANSPORT_SRC) $(LDFLAGS)

$(BUILD):
	mkdir -p $(BUILD)

clean:
	rm -rf $(BUILD)
