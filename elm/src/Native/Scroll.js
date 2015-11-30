Elm.Native.Scroll = {};
Elm.Native.Scroll.make = function(localRuntime) {
  localRuntime.Native = localRuntime.Native || {};
  localRuntime.Native.Scroll = localRuntime.Native.Scroll || {};
  if (localRuntime.Native.Scroll.values) {
    return localRuntime.Native.Scroll.values;
  }

  var NS = Elm.Native.Signal.make(localRuntime);

  var Task = Elm.Native.Task.make(localRuntime);
  var Utils = Elm.Native.Utils.make(localRuntime);

  var fromBottom = NS.input('Scroll.fromBottom', 0);

  localRuntime.addListener([fromBottom.id], window, 'scroll', function() {
    var scrolledHeight = window.pageYOffset + document.documentElement.clientHeight;

    localRuntime.notify(
      fromBottom.id,
      document.documentElement.scrollHeight - scrolledHeight
    );
  });

  function toBottom() {
    return Task.asyncFunction(function(callback) {
      window.scrollTo(0, document.body.scrollHeight);
      callback(Task.succeed(Utils.Tuple0));
    });
  }

  function scrollElement(id, delta) {
    return Task.asyncFunction(function(callback) {
      document.getElementById(id).scrollLeft -= delta;
      callback(Task.succeed(Utils.Tuple0));
    });
  }

  localRuntime.Native.Scroll.values = {
    toBottom: toBottom,
    fromBottom: fromBottom,
    scroll: F2(scrollElement),
  };

  return localRuntime.Native.Scroll.values;
};
