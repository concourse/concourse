var _concourse$atc$Native_Scroll = function() {
  function toBottom(id) {
    return _elm_lang$core$Native_Scheduler.nativeBinding(function(callback) {
      var ele = document.getElementById(id);
      ele.scrollTop = ele.scrollHeight - ele.clientHeight;
      callback(_elm_lang$core$Native_Scheduler.succeed(_elm_lang$core$Native_Utils.Tuple0));
    });
  }

  function fromBottom(id) {
    return _elm_lang$core$Native_Scheduler.nativeBinding(function(callback) {
      var ele = document.getElementById(id);
      var fromBottom = ele.scrollHeight - (ele.scrollTop + ele.clientHeight);
      callback(_elm_lang$core$Native_Scheduler.succeed(fromBottom));
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
    fromBottom: fromBottom,
    scrollElement: F2(scrollElement),
    scrollIntoView: scrollIntoView
  };
}();
