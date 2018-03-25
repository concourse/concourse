ELM_FILES = $(shell find src/ -type f -name '*.elm' -or -name '*.js')

all: search.js

.PHONY: clean

clean:
	rm -f search.js

search.js: $(ELM_FILES)
	elm make --warn --output $@ --yes src/Main.elm
