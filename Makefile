# Makefile includes src-d/ci methods and is used by Travis CI environment to build bblfsh-performance binaries,
# build and push docker image to a given registry.

# Package configuration
PROJECT = performance
COMMANDS = cmd/bblfsh-performance

GO_BUILD_ENV = CGO_ENABLED=0
DOCKER_ORG = bblfsh

# Including ci Makefile
CI_REPOSITORY ?= https://github.com/src-d/ci.git
CI_BRANCH ?= v1
CI_PATH ?= $(shell pwd)/.ci
MAKEFILE := $(CI_PATH)/Makefile.main
$(MAKEFILE):
	git clone --quiet --depth 1 -b $(CI_BRANCH) $(CI_REPOSITORY) $(CI_PATH);
-include $(MAKEFILE)
