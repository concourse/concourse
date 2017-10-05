ELM_FILES = $(shell find elm/src/ -type f -name '*.elm' -or -name '*.js')
ELM_TESTS_FILES = $(shell find elm/tests/ -type f -name '*.elm' -or -name '*.js')
LESS_FILES = $(shell find assets/css/ -type f -name '*.less')
PUBLIC_FILES = $(shell find public/ -type f)

all: public/elm.min.js public/main.css bindata.go

.PHONY: clean

clean:
	rm -f public/elm.js public/elm.min.js public/main.css bindata.go

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

bindata.go: $(PUBLIC_FILES)
	go-bindata ${DEV} -pkg web index.html public/...
	go fmt bindata.go

help:
	@ echo "$$HELP_INFO"

define HELP_INFO
Usage:
  make DEV=-dev: start development
  make DEV=-dev test : start development and testing
  make clean all : bundle code for production
endef
export HELP_INFO
