$(function () {
  if ($('.js-resource').length) {
    var pauseUnpause = new concourse.PauseUnpause(
      $('.js-resource'),
      function() {}, // on pause
      function() {}  // on unpause
    );
    pauseUnpause.bindEvents();
  }
});
