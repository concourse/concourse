ELM_FILES = $(shell find elm/src/ -type f -name '*.elm' -or -name '*.js')
LESS_FILES = $(shell find assets/css/ -type f -name '*.less')
PUBLIC_FILES = $(shell find public/ -type f)

all: public/elm.min.js public/main.css bindata.go

.PHONY: clean

clean:
	rm -f public/elm.js public/elm.min.js public/main.css bindata.go

public/elm.js: $(ELM_FILES)
	cd elm && elm make --warn --output ../public/elm.js --yes src/Main.elm

public/main.css: $(LESS_FILES)
	lessc --clean-css="--advanced" assets/css/main.less $@

public/elm.min.js: public/elm.js
	uglifyjs < $< > $@

bindata.go: $(PUBLIC_FILES)
	go-bindata ${DEV} -pkg web index.html public/...
	go fmt bindata.go
