var _concourse$atc$Native_Favicon = function() {
  function set(url) {
    return _elm_lang$core$Native_Scheduler.nativeBinding(function(callback) {
      var oldIcon = document.getElementById("favicon");
      var newIcon = document.createElement("link");
      newIcon.id = "favicon";
      newIcon.rel = "shortcut icon";
      newIcon.href = url;
      if (oldIcon) {
        document.head.removeChild(oldIcon);
      }

      document.head.appendChild(newIcon);

      callback(_elm_lang$core$Native_Scheduler.succeed(_elm_lang$core$Native_Utils.Tuple0));
    });
  }

  return {
    set: set
  };
}();
