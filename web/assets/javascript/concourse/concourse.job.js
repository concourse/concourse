$(function () {
  if ($('.js-job').length) {
    var pauseUnpause = new concourse.PauseUnpause($('.js-job'));
    pauseUnpause.bindEvents();
  }
});
