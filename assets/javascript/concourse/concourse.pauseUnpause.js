concourse.PauseUnpause = function ($el, pauseCallback, unpauseCallback) {
  this.$el = $el;
  this.pauseCallback = pauseCallback === undefined ? function(){} : pauseCallback;
  this.unpauseCallback = unpauseCallback === undefined ? function(){} : unpauseCallback;
  this.pauseBtn = this.$el.find('.js-pauseUnpause').pausePlayBtn();
  this.pauseEndpoint = this.$el.data('endpoint') + "/pause";
  this.unPauseEndpoint = this.$el.data('endpoint') + "/unpause";
  this.teamName = this.$el.data('teamname');
};

concourse.PauseUnpause.prototype.bindEvents = function () {
  var _this = this;

  _this.$el.delegate('.js-pauseUnpause.disabled', 'click', function (event) {
    _this.pause();
  });

  _this.$el.delegate('.js-pauseUnpause.enabled', 'click', function (event) {
    _this.unpause();
  });
};

concourse.PauseUnpause.prototype.pause = function (pause) {
  var _this = this;
  _this.pauseBtn.loading();

  $.ajax({
    method: 'PUT',
    url: _this.pauseEndpoint,
  }).done(function (resp, jqxhr) {
    _this.pauseBtn.enable();
    _this.pauseCallback();
  }).error(function (resp) {
    _this.requestError(resp);
  });
};


concourse.PauseUnpause.prototype.unpause = function (event) {
  var _this = this;
  _this.pauseBtn.loading();

  $.ajax({
    method: 'PUT',
    url: _this.unPauseEndpoint
  }).done(function (resp) {
    _this.pauseBtn.disable();
    _this.unpauseCallback();
  }).error(function (resp) {
    _this.requestError(resp);
  });
};

concourse.PauseUnpause.prototype.requestError = function (resp) {
  this.pauseBtn.error();

  if (resp.status == 401) {
    concourse.redirect("/teams/" + this.teamName + "/login");
  }
};
