.PHONY: build pack android-test connected-android-test test clean

EXPORTS_PATH ?= ../../exports
include $(EXPORTS_PATH)/common.mk

ifeq ($(NO_DAEMON),1)
override GRADLE_FLAGS += --no-daemon
endif

build:
	./gradlew $(GRADLE_FLAGS) assembleRelease

pack: build

# Unit tests use library for desktop OS platform, so we still need .try-build-lib for this
# The unit tests find this library through the library path set in common.mk
unit-test: .try-build-lib
	./gradlew $(GRADLE_FLAGS) test

android-test:
	./gradlew $(GRADLE_FLAGS) pixel2DebugAndroidTest

connected-android-test:
	./gradlew $(GRADLE_FLAGS) connectedAndroidTest

test: .try-build-lib
ifeq ($(NO_EMULATOR),1)
	./gradlew $(GRADLE_FLAGS) test
else
	./gradlew $(GRADLE_FLAGS) test pixel2DebugAndroidTest
endif

clean:
	rm -rf lib/build lib/.cxx lib/CMakeFiles lib/src/test/resources/*
ifeq ($(CLEAN_ALL),1)
	rm -rf .gradle/
endif
