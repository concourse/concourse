var _concourse$atc$Native_Redirect = function() {
  function to(url) {
    return _elm_lang$core$Native_Scheduler.nativeBinding(function(callback) {
      window.location = url;
      callback(_elm_lang$core$Native_Scheduler.succeed(_elm_lang$core$Native_Utils.Tuple0));
    });
  }

  return {
    to: to
  };
}();
