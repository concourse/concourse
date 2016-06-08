var _concourse$atc$Native_EventSource = function() {
  function open(url, settings) {
    return _elm_lang$core$Native_Scheduler.nativeBinding(function(callback) {
      var source = new EventSource(url);

      function dispatchEvent(event) {
        var ev = {
          data: event.data
        }

        if (event.type !== undefined) {
          ev.name = _elm_lang$core$Maybe$Just(event.type);
        } else {
          ev.name = _elm_lang$core$Maybe$Nothing;
        }

        if (event.lastEventId !== undefined) {
          ev.lastEventId = _elm_lang$core$Maybe$Just(event.lastEventId);
        } else {
          ev.lastEventId = _elm_lang$core$Maybe$Nothing;
        }

        _elm_lang$core$Native_Scheduler.rawSpawn(settings.onEvent(ev));
      };

      source.onmessage = function(event) {
        dispatchEvent(event);
      };

      _elm_lang$core$Native_List.toArray(settings.events).forEach(function(eventType) {
        source.addEventListener(eventType, function(event) {
          dispatchEvent(event);
        });
      });

      source.onopen = function(event) {
        _elm_lang$core$Native_Scheduler.rawSpawn(settings.onOpen(source));
      };

      source.onerror = function(event) {
        _elm_lang$core$Native_Scheduler.rawSpawn(settings.onError(_elm_lang$core$Native_Utils.Tuple0));
      };
    });
  }

  function close(source) {
    return _elm_lang$core$Native_Scheduler.nativeBinding(function(callback) {
      source.close();
      callback(_elm_lang$core$Native_Scheduler.succeed(_elm_lang$core$Native_Utils.Tuple0));
    });
  }

  return {
    open: F2(open),
    close: close
  };
}();
