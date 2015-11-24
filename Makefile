ELM_FILES = $(shell find elm/ -type f -name '*.elm' -or -name '*.js')
LESS_FILES = $(shell find assets/css/ -type f -name '*.less')

all: public/elm.min.js public/main.css

.PHONY: clean

clean:
	rm -f public/elm.js public/elm.min.js public/main.css

public/elm.js: $(ELM_FILES)
	cd elm && elm make --warn --output ../public/elm.js --yes src/Main.elm

public/main.css: $(LESS_FILES)
	lessc --clean-css="--advanced" assets/css/main.less $@

public/elm.min.js: public/elm.js
	uglifyjs < $< > $@
