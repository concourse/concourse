var browserify = require('browserify');
var gulp = require('gulp');
var gutil = require('gulp-util');
var source = require("vinyl-source-stream");
var reactify = require('reactify');
var watchify = require('watchify');
var uglify = require('gulp-uglify');
var buffer = require('vinyl-buffer');
var concat = require('gulp-concat');
var jshint = require('gulp-jshint');
var addsrc = require('gulp-add-src');
var jasmineBrowser = require('gulp-jasmine-browser');
var less = require('gulp-less');
var path = require('path');
var minifyCSS = require('gulp-minify-css');

var production = (process.env.NODE_ENV === 'production');
var publicDir = "../public"

function rebundle(bundler) {
  var stream = bundler.bundle().
    on('error', gutil.log.bind(gutil, 'Browserify Error')).
    pipe(source(publicDir + '/build.js'));

  if (production) {
    stream = stream.pipe(buffer()).pipe(uglify());
  }

  stream.pipe(gulp.dest('.'));
}

gulp.task('compile-build', function () {
  var bundler = browserify('./javascript/event_handler.jsx', { debug: !production });
  bundler.transform(reactify);

  return rebundle(bundler);
});

gulp.task('compile-concourse', function () {
   var stream = gulp.src(["javascript/concourse/concourse.js", "javascript/concourse/concourse.*.js", "javascript/concourse/jquery.*.js"])
    .pipe(jshint())
    .pipe(jshint.reporter('jshint-stylish'))
    .pipe(concat('concourse.js'))

    if (production) {
      stream = stream.pipe(buffer()).pipe(uglify());
    }

    return stream.pipe(gulp.dest(publicDir));
});

// jasmine stuff
var externalFiles = [publicDir + "/jquery-2.1.1.min.js", "javascript/spec/helpers/**/*.js"]
var jsSourceFiles = ["javascript/concourse/concourse.js", "javascript/concourse/concourse.*.js", "javascript/concourse/jquery.*.js", "javascript/spec/**/*_spec.js"]
var hintSpecFiles = function() {
  gulp.src('javascript/spec/**/*_spec.js')
}

gulp.task('jasmine-cli', function(cb) {
  return gulp.src(jsSourceFiles)
    .pipe(jshint())
    .pipe(jshint.reporter('jshint-stylish'))
    .on('error', process.exit.bind(process, 1))
    .pipe(addsrc(externalFiles))
    .pipe(jasmineBrowser.specRunner({console: true}))
    .pipe(jasmineBrowser.phantomjs())
    .on('error', process.exit.bind(process, 1));
});

gulp.task('jasmine', function() {
  return gulp.src(jsSourceFiles)
    .pipe(addsrc(externalFiles))
    .pipe(jasmineBrowser.specRunner())
    .pipe(jasmineBrowser.server());
});

gulp.task('compile-css', function() {

  return gulp.src('css/main.less')
    .pipe(less())
    .pipe(minifyCSS())
    .pipe(gulp.dest(publicDir));

})

gulp.task('watch', function () {
  var bundler = watchify(browserify('./javascript/event_handler.jsx'), { debug: !production });
  bundler.transform(reactify);

  bundler.on('update', function() { rebundle(bundler); });

  return rebundle(bundler);
});


gulp.task('default', ['compile-build', 'compile-concourse', 'compile-css']);
