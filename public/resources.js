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
        $(that).closest("tr").removeClass("disabled").addClass("enabled");
      } else {
        $(that).data("action", "enable");
        $(that).closest("tr").removeClass("enabled").addClass("disabled");
      }
    });

    return false;
  });
});
