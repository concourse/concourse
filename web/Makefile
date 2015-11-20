ELM_FILES = $(shell find elm/ -type f -name '*.elm')

public/elm.js: $(ELM_FILES)
	cd elm && elm make --warn --output ../public/elm.js --yes src/Main.elm
