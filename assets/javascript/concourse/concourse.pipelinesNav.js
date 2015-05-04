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
      $pipelineListItem.html('<a href="' + pipeline.url + '">' + pipeline.name + '</a>');

      _this.$list.append($pipelineListItem);
    });
  });
};

$(function () {
  if ($('.js-pipelinesNav').length) {
    var pipelinesNav = new concourse.PipelinesNav($('.js-pipelinesNav'));
    pipelinesNav.bindEvents();
  }
});
