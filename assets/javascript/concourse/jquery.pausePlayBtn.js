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
