var browserify = require('browserify');
var gulp = require('gulp');
var gutil = require('gulp-util');
var source = require("vinyl-source-stream");
var reactify = require('reactify');
var watchify = require('watchify');
var uglify = require('gulp-uglify');
var buffer = require('vinyl-buffer');

var production = (process.env.NODE_ENV === 'production');

function rebundle(bundler) {
  var stream = bundler.bundle().
    on('error', gutil.log.bind(gutil, 'Browserify Error')).
    pipe(source('../public/build.js'));

  if (production) {
    stream = stream.pipe(buffer()).pipe(uglify());
  }

  stream.pipe(gulp.dest('.'));
}

gulp.task('build', function () {
  var bundler = browserify('./build.jsx', { debug: !production });
  bundler.transform(reactify);

  return rebundle(bundler);
});

gulp.task('watch', function () {
  var bundler = watchify(browserify('./build.jsx'), { debug: !production });
  bundler.transform(reactify);

  bundler.on('update', function() { rebundle(bundler); });

  return rebundle(bundler);
});

gulp.task('default', ['build']);
