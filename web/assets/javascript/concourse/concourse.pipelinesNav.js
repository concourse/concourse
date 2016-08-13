(function(){
  concourse.PipelinesNav = function ($el) {
    this.$el = $($el);
    this.$toggle = $el.find($('.js-sidebar-toggle'));
  };

  concourse.PipelinesNav.prototype.bindEvents = function () {
    var _this = this;
    _this.$toggle.on("click", function() {
        _this.toggle();
    });
  };

  concourse.PipelinesNav.prototype.toggle = function() {
    $('.js-sidebar').toggleClass('visible');
  };
})();

$(function () {
  if ($('.js-with-pipeline').length) {
    var withPipeline = new concourse.PipelinesNav($('.js-with-pipeline'));
    withPipeline.bindEvents();
  }
});
