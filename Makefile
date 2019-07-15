all: js/search.js css/booklit.css css/blog.css css/pipeline.css

.PHONY: clean

clean:
	rm -f js/search.js
	rm -f css/booklit.css
	rm -f css/blog.css
	rm -f css/pipeline.css

js/search.js: elm/Search.elm elm/Query.elm
	yarn run elm make --output $@ $^

css/booklit.css: less/booklit.less less/*.less
	yarn run lessc $< $@

css/blog.css: less/blog.less less/*.less
	yarn run lessc $< $@

css/pipeline.css: less/pipeline.less
	yarn run lessc $< $@

blog/concourse.zip: blog/package.json blog/*.hbs css/*.css images/* blog/partials/*.hbs
	cp css/*.css blog/assets/css/
	cp -r images/* blog/assets/images/
	gscan ./blog
	cd blog && zip -r concourse.zip package.json *.hbs assets partials
