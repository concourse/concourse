$(function () {
  if ($('.js-resource').length) {
    var pauseUnpause = new concourse.PauseUnpause($('.js-resource'), function() {
      $('.js-resource').find('.js-resourceStatusText').html("checking paused");
    }, function() {
      $('.js-resource').find('.js-resourceStatusText').html("checking successfully");
    });
    pauseUnpause.bindEvents();
  }
});
