objects = js/search.js css/booklit.css css/blog.css css/prism.css css/pipeline.css discourse/common/common.scss discourse/desktop/desktop.scss discourse/mobile/mobile.scss blog/concourse.zip

all: $(objects)

.PHONY: clean

clean:
	rm -f $(objects)

js/search.js: elm/Search.elm elm/Query.elm
	yarn run elm make --output $@ $^

css/booklit.css: less/booklit.less less/*.less
	yarn run lessc $< $@

css/blog.css: less/blog.less less/*.less
	yarn run lessc $< $@

css/prism.css: less/prism.less less/*.less
	yarn run lessc $< $@

css/pipeline.css: less/pipeline.less
	yarn run lessc $< $@

discourse/common/common.scss: less/discourse/common.less less/*.less
	yarn run lessc $< $@

discourse/desktop/desktop.scss: less/discourse/desktop.less less/*.less
	yarn run lessc $< $@

discourse/mobile/mobile.scss: less/discourse/mobile.less less/*.less
	yarn run lessc $< $@

blog/concourse.zip: blog/package.json blog/*.hbs css/*.css images/* blog/partials/*.hbs
	cp css/*.css blog/assets/css/
	cp js/prism.js blog/assets/js/
	cp -r images/* blog/assets/images/
	yarn run gscan ./blog
	cd blog && zip -r concourse.zip package.json *.hbs assets partials
