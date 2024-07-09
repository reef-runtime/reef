.PHONY: build

#
# Regenerate all GO capnp files.
#

build:
	make -C ./node/ build
	make -C ./compiler/ build

#
# Test if code can be generated and that it can compile.
#

test:
	make -C ./node/ test
	make -C ./compiler/ test

