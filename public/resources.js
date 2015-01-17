$(document).ready(function() {
  $("a.toggle-resource-version").on("click", function() {
    var target;

    if ($(this).text() == "enable") {
      target = $(this).data("enable-url");
    } else {
      target = $(this).data("disable-url");
    }

    var that = this;

    $.ajax({
      method: "PUT",
      url: target
    }).done(function() {
      if ($(that).text() == "enable") {
        $(that).text("disable");
        $(that).closest("tr").removeClass("disabled").addClass("enabled");
      } else {
        $(that).text("enable");
        $(that).closest("tr").removeClass("enabled").addClass("disabled");
      }
    });

    return false;
  });
});
