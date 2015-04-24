concourse.Job = function ($el) {
  this.$el = $el;
  this.pauseBtn = this.$el.find('.js-pauseJobCheck').pausePlayBtn();
  this.jobName = this.$el.data('job-name');
  this.pauseEndpoint = "/api/v1/pipelines/" + window.pipelineName + "/jobs/" + this.jobName + "/pause";
  this.unPauseEndpoint = "/api/v1/pipelines/" + window.pipelineName + "/jobs/" + this.jobName + "/unpause";
};

concourse.Job.prototype.bindEvents = function () {
  var _this = this;

  _this.$el.delegate('.js-pauseJobCheck.disabled', 'click', function (event) {
    _this.pause();
  });

  _this.$el.delegate('.js-pauseJobCheck.enabled', 'click', function (event) {
    _this.unpause();
  });
};

concourse.Job.prototype.pause = function (pause) {
  var _this = this;
  _this.pauseBtn.loading();

  $.ajax({
    method: 'PUT',
    url: _this.pauseEndpoint,
  }).done(function (resp, jqxhr) {
    _this.pauseBtn.enable();
  }).error(function (resp) {
    _this.pauseBtn.error();
  });
};

concourse.Job.prototype.unpause = function (event) {
  var _this = this;
  _this.pauseBtn.loading();

  $.ajax({
    method: 'PUT',
    url: this.unPauseEndpoint
  }).done(function (resp) {
    _this.pauseBtn.disable();
  }).error(function (resp) {
    _this.pauseBtn.error();
  });
};

$(function () {
  if ($('.js-job').length) {
    var job = new concourse.Job($('.js-job'));
    job.bindEvents();
  }
});
