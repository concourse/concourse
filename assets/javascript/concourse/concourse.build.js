concourse.Build = function ($el) {
  this.$el = $el;
  this.$abortBtn = this.$el.find('.js-abortBuild');
  this.buildID = this.$el.data('build-id');
  this.abortEndpoint = '/api/v1/builds/' + this.buildID + '/abort';
};

concourse.Build.prototype.bindEvents = function () {
  var _this = this;
  this.$abortBtn.on('click', function(event) {
    _this.abort();
  });
};

concourse.Build.prototype.abort = function() {
  var _this = this;

  $.ajax({
    method: 'POST',
    url: _this.abortEndpoint
  }).done(function (resp, jqxhr) {
    _this.$abortBtn.remove();
  }).error(function (resp) {
    _this.$abortBtn.addClass('errored');
  });
};

$(function () {
  if ($('.js-build').length) {
    var build = new concourse.Build($('.js-build'));
    build.bindEvents();
  }
});
