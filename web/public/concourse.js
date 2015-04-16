var concourse = {};

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
    url: _this.abortEndpoint,
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

concourse.Job = function ($el) {
  this.$el = $el;
  this.pauseBtn = this.$el.find('.js-pauseJobCheck').pausePlayBtn();
  this.jobName = this.$el.data('job-name');
  this.pauseEndpoint = "/api/v1/jobs/" + this.jobName + "/pause";
  this.unPauseEndpoint = "/api/v1/jobs/" + this.jobName + "/unpause";
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

// <button class="btn-pause disabled js-pauseResourceCheck"><i class="fa fa-fw fa-pause"></i></button>

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
