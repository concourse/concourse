ELM_FILES = $(shell find web/elm/src/ -type f -name '*.elm' -or -name '*.js')
ELM_TESTS_FILES = $(shell find web/elm/tests/ -type f -name '*.elm' -or -name '*.js')
LESS_FILES = $(shell find web/assets/css/ -type f -name '*.less')

PATH := $(PWD)/node_modules/.bin:$(PATH)

all: web/public/elm.min.js web/public/main.css

.PHONY: clean
clean:
	rm -f web/public/elm.js web/public/elm.min.js web/public/main.css

web/public/elm.js: $(ELM_FILES) $(ELM_TESTS_FILES)
	cd web/elm && elm make --warn --output ../public/elm.js --yes src/Main.elm

web/public/main.css: $(LESS_FILES)
	lessc --clean-css="--advanced" web/assets/css/main.less $@

web/public/elm.min.js: web/public/elm.js
	uglifyjs < $< > $@
