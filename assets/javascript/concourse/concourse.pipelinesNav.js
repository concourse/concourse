concourse.PipelinesNav = function ($el) {
  this.$el = $($el);
  this.$toggle = $el.find($('.js-pipelinesNav-toggle'));
  this.$list = $el.find($('.js-pipelinesNav-list'));
  this.pipelinesEndpoint = '/api/v1/pipelines';
};


concourse.PipelinesNav.prototype.bindEvents = function () {
  var _this = this;
  _this.$toggle.on("click", function() {
      _this.toggle();
  });

  _this.loadPipelines();
};

concourse.PipelinesNav.prototype.toggle = function() {
  $('body').toggleClass('pipelinesNav-visible');
};

concourse.PipelinesNav.prototype.loadPipelines = function() {
  var _this = this;
  $.ajax({
    method: 'GET',
    url: _this.pipelinesEndpoint
  }).done(function(resp, jqxhr){
    $(resp).each( function(index, pipeline){
      var $pipelineListItem = $("<li>");

      var ed = pipeline.paused ? 'enabled' : 'disabled';
      var icon = pipeline.paused ? 'play' : 'pause';

      $pipelineListItem.html('<span class="btn-pause fl ' + ed + ' js-pauseUnpause"><i class="fa fa-fw fa-' + icon +  '"></i></span><a href="' + pipeline.url + '">' + pipeline.name + '</a>');
      $pipelineListItem.data('endpoint', 'pipelines/' + pipeline.name);
      $pipelineListItem.data('pipelineName', pipeline.name);
      $pipelineListItem.addClass('clearfix');


      _this.$list.append($pipelineListItem);

      _this.newPauseUnpause($pipelineListItem);

      if(window.pipelineName === pipeline.name && pipeline.paused) {
        _this.$el.find('.js-groups').addClass('paused');
      }
    });
  });
};

concourse.PipelinesNav.prototype.newPauseUnpause = function($el) {
  var _this = this;
  var pauseUnpause = new concourse.PauseUnpause($el, function() {
    if($el.data('pipelineName') === window.pipelineName) {
      _this.$el.find('.js-groups').addClass('paused');
    }
  }, function() {
    if($el.data('pipelineName') === window.pipelineName) {
      _this.$el.find('.js-groups').removeClass('paused');
    }
  });
  pauseUnpause.bindEvents();
};

$(function () {
  if ($('.js-pipelinesNav').length) {
    var pipelinesNav = new concourse.PipelinesNav($('.js-pipelinesNav'));
    pipelinesNav.bindEvents();
  }
});
