var _concourse$atc$Native_Scroll = function() {
  function toBottom() {
    return _elm_lang$core$Native_Scheduler.nativeBinding(function(callback) {
      window.scrollTo(0, document.body.scrollHeight);
      callback(_elm_lang$core$Native_Scheduler.succeed(_elm_lang$core$Native_Utils.Tuple0));
    });
  }

  function scrollElement(id, delta) {
    return _elm_lang$core$Native_Scheduler.nativeBinding(function(callback) {
      document.getElementById(id).scrollLeft -= delta;
      callback(_elm_lang$core$Native_Scheduler.succeed(_elm_lang$core$Native_Utils.Tuple0));
    });
  }

  function scrollIntoView(selector) {
    return _elm_lang$core$Native_Scheduler.nativeBinding(function(callback) {
      document.querySelector(selector).scrollIntoView();
      callback(_elm_lang$core$Native_Scheduler.succeed(_elm_lang$core$Native_Utils.Tuple0));
    });
  }

  return {
    toBottom: toBottom,
    scrollElement: F2(scrollElement),
    scrollIntoView: scrollIntoView
  };
}();
