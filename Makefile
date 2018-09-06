ELM_FILES = $(shell find elm/src/ -type f -name '*.elm' -or -name '*.js')
ELM_TESTS_FILES = $(shell find elm/tests/ -type f -name '*.elm' -or -name '*.js')
LESS_FILES = $(shell find assets/css/ -type f -name '*.less')
PUBLIC_FILES = $(shell find public/ -type f)

all: public/elm.min.js public/main.css

.PHONY: clean

clean:
	rm -f public/elm.js public/elm.min.js public/main.css

public/elm.js: $(ELM_FILES) $(ELM_TESTS_FILES)
	cd elm && elm make --warn --output ../public/elm.js --yes src/Main.elm

public/main.css: $(LESS_FILES)
	lessc --clean-css="--advanced" assets/css/main.less $@

public/elm.min.js: public/elm.js
	uglifyjs < $< > $@

test: all
	cd elm && elm-test

test-watch: all
	cd elm && elm-test --watch
