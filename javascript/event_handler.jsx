var React = require('react/addons');

var Build = require('./build.jsx');

var $ = require('jquery');

var moment = require('moment');
require("moment-duration-format");

var flux = require('./flux');

var buildComponent = <Build flux={flux}/>;

function renderBuildComponent() {
  var rendered = React.render(
    buildComponent,
    document.getElementById('build-logs')
  );

  var title = $("#page-header");

  if (title.hasClass("pending") || title.hasClass("started")) {
    rendered.setState({ autoscroll: true });
  }

  $(window).scroll(function() {
    var scrollEnd = $(window).scrollTop() + $(window).height();

    if (scrollEnd >= ($(document).height() - 16)) {
      rendered.setState({ autoscroll: true });
    } else {
      rendered.setState({ autoscroll: false });
    }
  });
}

function streamLog(uri, status) {
  var es = new EventSource(uri);

  var renderImmediately = status == 'pending' || status == 'started';

  if (renderImmediately) {
    renderBuildComponent();
  }

  var successfullyConnected = false;

  es.addEventListener("event", function(event) {
    var msg = JSON.parse(event.data);

    var versionSegments = msg.version.split(".");

    var majorHandler = eventHandlers[versionSegments[0]];
    if (!majorHandler) {
      console.log("unknown major version: " + msg.version);
      return;
    }

    var minorHandler = majorHandler[versionSegments[1]] || majorHandler["*"];
    if (!minorHandler) {
      console.log("unknown minor version: " + msg.version);
      return;
    }

    var handler = minorHandler[msg.event];
    if (!handler) {
      return;
    }

    handler(msg.data);
  });

  es.addEventListener("end", function(event) {
    es.close();

    if (!renderImmediately) {
      renderBuildComponent();
    }
  });

  es.onopen = function() {
    successfullyConnected = true;
  };

  es.onerror = function(event) {
    if(!successfullyConnected) {
      // assume rejected because of auth
      $("#build-requires-auth").show();
    }
  };
}

var eventHandlers = {
  "1": {
    "*": {
      "log": function(data) {
        processLogs(data);
      },

      "error": function(data) {
        processError(data);
      },

      "status": function(data) {
        processStatus(data);
      },

      "input": function(data) {
        var resource = event["input"];

        var origin = legacyInputOrigin(resource.name);

        flux.actions.setStepVersionInfo(origin, resource.version, resource.metadata);
        flux.actions.setStepRunning(origin, false);
      },

      "output": function(data) {
        var resource = event["output"];

        var origin = legacyOutputOrigin(resource.name);

        flux.actions.setStepVersionInfo(origin, resource.version, resource.metadata);
        flux.actions.setStepRunning(origin, false);
      },

      "finish": function(data) {
        var origin = legacyRunOrigin();
        flux.actions.setStepRunning(origin, false);
      },

      "initialize-task": function(data) {
        flux.actions.setStepRunning(data.origin, true);
      },

      "finish-task": function(data) {
        flux.actions.setStepSuccessful(data.origin, data.exit_status == 0);
        flux.actions.setStepRunning(data.origin, false);
      },

      "finish-get": function(data) {
        flux.actions.setStepVersionInfo(data.origin, data.version, data.metadata);
        flux.actions.setStepRunning(data.origin, false);
      },

      "finish-put": function(data) {
        flux.actions.setStepVersionInfo(data.origin, data.version, data.metadata);
        flux.actions.setStepRunning(data.origin, false);
      }
    }
  },

  "2": {
    "*": {
      "input": function(data) {
        var origin = legacyInputOrigin(data.plan.name);

        flux.actions.setStepVersionInfo(origin, data.version, data.metadata);
        flux.actions.setStepRunning(origin, false);
      },

      "output": function(data) {
        var origin = legacyOutputOrigin(data.plan.name);

        flux.actions.setStepVersionInfo(origin, data.version, data.metadata);
        flux.actions.setStepRunning(origin, false);
      },

      "log": function(data) {
        processLogs(data);
      },
    }
  }
}

function processStatus(event) {
  var currentStatus = $("#page-header").attr("class");

  var buildTimes = $(".build-times");

  var status = event.status;
  var m = moment.unix(event.time);

  var time = $("<time>");
  time.text(m.fromNow());
  time.attr("datetime", m.format());
  time.attr("title", m.format("lll Z"));

  if(status == "started") {
    time.addClass('js-startTime');
    $("<dt/>").text(status).appendTo(buildTimes);
    $("<dd/>").append(time).appendTo(buildTimes);
  } else {
    $("<dt/>").text(status).appendTo(buildTimes);
    $("<dd/>").append(time).appendTo(buildTimes);

    var startTime = $(".js-startTime").attr("datetime");

    // Some events cause the build to never start (e.g. input errors).
    var didStart = !!startTime

    if(didStart) {
      var duration = moment.duration(m.diff(moment(startTime)));

      var durationEle = $("<span>");
      durationEle.addClass("duration");
      durationEle.text(duration.format("h[h]m[m]s[s]"));

      $("<dt/>").text("duration").appendTo(buildTimes);
      $("<dd/>").append(durationEle).appendTo(buildTimes);
    }
  }

  // only transition from transient states; state may already be set
  // if the page loaded after build was done
  if(currentStatus != "pending" && currentStatus != "started") {
    return;
  }

  $("#page-header").attr("class", status);
  $("#builds .current").attr("class", status + " current");

  if(status != "started") {
    $(".abort-build").remove();
  }
}

function processLogs(event) {
  var origin;

  switch(event.origin.type) {
  case "run":
    origin = legacyRunOrigin();
    break;
  case "input":
    origin = legacyInputOrigin(event.origin.name);
    break;
  case "output":
    origin = legacyOutputOrigin(event.origin.name);
    break;
  case "get":
  case "put":
  case "task":
    origin = event.origin;
    break;
  }

  if(!origin || !event.payload) {
    return;
  }

  flux.actions.setStepRunning(origin, true);
  flux.actions.addLog(origin, event.payload);
}

function processError(event) {
  var origin;

  if(event.origin && event.origin.type) {
    switch(event.origin.type) {
    case "input":
      origin = legacyOutputOrigin(event.origin.name);
      break;
    case "output":
      origin = legacyOutputOrigin(event.origin.name);
      break;
    case "get":
    case "put":
    case "task":
      origin = event.origin;
      break;
    }
  } else {
    origin = legacyRunOrigin();
  }

  if(!origin) {
    return;
  }

  flux.actions.setStepRunning(origin, false);
  flux.actions.setStepErrored(origin, true);
  flux.actions.addError(origin, event.message);
}

function scrollToCurrentBuild() {
  var currentBuild = $("#builds .current");
  var buildWidth = currentBuild.width();
  var left = currentBuild.offset().left;

  if((left + buildWidth) > window.innerWidth) {
    $("#builds").scrollLeft(left - buildWidth);
  }
}

$(document).ready(function() {
  $("#builds").bind('mousewheel', function(e){
    if (e.originalEvent.deltaX != 0) {
      $(this).scrollLeft($(this).scrollLeft() + e.originalEvent.deltaX);
    } else {
      $(this).scrollLeft($(this).scrollLeft() - e.originalEvent.deltaY);
    }

    return false;
  });

  scrollToCurrentBuild();
});

var legacyInputLocations = [];
var legacyOutputLocations = [];

function legacyInputOrigin(name) {
  var inputOrigin = legacyInputLocations.indexOf(name);
  if (inputOrigin == -1) {
    inputOrigin = legacyInputLocations.length;
    legacyInputLocations.push(name);
  }

  var loc = [0, inputOrigin];

  return {
    "name": name,
    "type": "get",
    "location": loc
  }
}

function legacyRunOrigin() {
  return {
    "name": "build",
    "type": "task",
    "location": [1]
  }
}

function legacyOutputOrigin(name) {
  var outputOrigin = legacyOutputLocations.indexOf(name);
  if (outputOrigin == -1) {
    outputOrigin = legacyOutputLocations.length;
    legacyOutputLocations.push(name);
  }

  var loc = [2, outputOrigin];

  return {
    "name": name,
    "type": "put",
    "location": loc
  }
}

window.streamLog = streamLog;
window.preloadInput = flux.actions.preloadInput;
