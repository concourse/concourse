$(document).ready(function() {
  $(".js-toggleResource").on("click", function() {
    var target;

    if ($(this).data("action") == "enable") {
      target = $(this).data("enable-url");
    } else {
      target = $(this).data("disable-url");
    }

    var that = this;

    $.ajax({
      method: "PUT",
      url: target
    }).done(function() {
      if ($(that).data("action") == "enable") {
        $(that).data("action", "disable");
        $(that).closest("li").removeClass("disabled").addClass("enabled");
      } else {
        $(that).data("action", "enable");
        $(that).closest("li").removeClass("enabled").addClass("disabled");
      }
    }).error(function(resp) {
      if (resp.status == 401) {
        window.location = "/login";
      }

      if ($(that).data("action") == "enable") {
        $(that).closest("li").removeClass("disabled").addClass("errored");
      } else {
        $(that).closest("li").removeClass("enabled").addClass("errored");
      }
    });

    return false;
  });
});
