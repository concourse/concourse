DOCUMENTS := $(shell find $(SOURCEDIR) -name '*.any')

all: out

out: $(DOCUMENTS)
	RUBYLIB=${PWD}/lib anatomy -i concourse.any -o out
