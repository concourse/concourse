(function ($) {
    $.fn.pausePlayBtn = function () {
      var $el = $(this);
      return {
        loading: function() {
          $el.removeClass('disabled enabled').addClass('loading');
          $el.find('i').removeClass('fa-pause').addClass('fa-circle-o-notch fa-spin');
        },

        enable: function() {
          $el.removeClass('loading').addClass('enabled');
          $el.find('i').removeClass('fa-circle-o-notch fa-spin').addClass('fa-play');
        },

        error: function() {
          $el.removeClass('loading').addClass('errored');
          $el.find('i').removeClass('fa-circle-o-notch fa-spin').addClass('fa-pause');
        },

        disable: function() {
          $el.removeClass('loading').addClass('disabled');
          $el.find('i').removeClass('fa-circle-o-notch fa-spin').addClass('fa-pause');
        }
      };
    };
})(jQuery);

concourse.Resource = function ($el) {
  this.$el = $el;
  this.pauseBtn = this.$el.find('.js-pauseResourceCheck').pausePlayBtn();
  this.resourceName = this.$el.data('resource-name');
  this.pauseEndpoint = "/api/v1/resources/" + this.resourceName + "/pause";
  this.unPauseEndpoint = "/api/v1/resources/" + this.resourceName + "/unpause";
};

concourse.Resource.prototype.bindEvents = function () {
  var _this = this;

  _this.$el.delegate('.js-pauseResourceCheck.disabled', 'click', function (event) {
    _this.pause();
  });

  _this.$el.delegate('.js-pauseResourceCheck.enabled', 'click', function (event) {
    _this.unpause();
  });
};

concourse.Resource.prototype.pause = function (pause) {
  var _this = this;
  _this.pauseBtn.loading();

  $.ajax({
    method: 'PUT',
    url: _this.pauseEndpoint,
  }).done(function (resp, jqxhr) {
    _this.pauseBtn.enable();

    _this.$el.find('.js-resourceStatusText').html("checking paused");
  }).error(function (resp) {
    _this.pauseBtn.error();
  });
};

concourse.Resource.prototype.unpause = function (event) {
  var _this = this;
  _this.pauseBtn.loading();

  $.ajax({
    method: 'PUT',
    url: this.unPauseEndpoint
  }).done(function (resp) {
    _this.pauseBtn.disable();

    _this.$el.find('.js-resourceStatusText').html("checking successfully");
  }).error(function (resp) {
    _this.pauseBtn.error();
  });
};

$(function () {
  if ($('.js-resource').length) {
    var resource = new concourse.Resource($('.js-resource'));
    resource.bindEvents();
  }
});
