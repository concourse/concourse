all: js/search.js css/booklit.css css/pipeline.css

.PHONY: clean

clean:
	rm -f js/search.js
	rm -f css/booklit.css
	rm -f css/pipeline.css

js/search.js: elm/Search.elm elm/Query.elm
	yarn run elm make --output $@ $^

css/booklit.css: less/booklit.less less/responsive.less
	yarn run lessc $< $@

css/pipeline.css: less/pipeline.less
	yarn run lessc $< $@
