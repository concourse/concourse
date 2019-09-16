var resizeTimer;

app.ports.pinTeamNames.subscribe(function(config) {
  sections = () => Array.from(document.querySelectorAll("." + config.sectionClass));
  header = (section) => Array.from(section.childNodes).find(n => n.classList && n.classList.contains(config.sectionHeaderClass));
  body = (section) => Array.from(section.childNodes).find(n => n.classList && n.classList.contains(config.sectionBodyClass));
  lowestHeaderTop = (section) => body(section).offsetTop + body(section).scrollHeight - header(section).scrollHeight;

  pageHeaderHeight = () => config.pageHeaderHeight;
  viewportTop = () => window.pageYOffset + pageHeaderHeight();

  updateHeader = (section) => {
    var scrolledFarEnough = section.offsetTop < viewportTop();
    var scrolledTooFar = lowestHeaderTop(section) < viewportTop();
    if (!scrolledFarEnough && !scrolledTooFar) {
      header(section).style.top = "";
      header(section).style.position = "";
      body(section).style.paddingTop = "";
      return 'static';
    } else if (scrolledFarEnough && !scrolledTooFar) {
      header(section).style.position = 'fixed';
      header(section).style.top = pageHeaderHeight() + "px";
      body(section).style.paddingTop = header(section).scrollHeight + "px";
      return 'fixed';
    } else if (scrolledFarEnough && scrolledTooFar) {
      header(section).style.position = 'absolute';
      header(section).style.top = lowestHeaderTop(section) + "px";
      return 'absolute';
    } else if (!scrolledFarEnough && scrolledTooFar) {
      return 'impossible';
    }
  }

  updateSticky = () => {
    document.querySelector("." + config.pageBodyClass).style.marginTop = pageHeaderHeight();
    sections().forEach(updateHeader);
  }

  clearTimeout(resizeTimer);
  resizeTimer = setTimeout(updateSticky, 250);
  window.onscroll = updateSticky;
});

app.ports.resetPipelineFocus.subscribe(resetPipelineFocus);

app.ports.renderPipeline.subscribe(function (values) {
  setTimeout(function(){ // elm 0.17 bug, see https://github.com/elm-lang/core/issues/595
    foundSvg = d3.select(".pipeline-graph");
    var svg = createPipelineSvg(foundSvg)
    if (svg.node() != null) {
      var jobs = values[0];
      var resources = values[1];
      draw(svg, jobs, resources, app.ports.newUrl);
    }
  }, 0)
});

app.ports.requestLoginRedirect.subscribe(function (message) {
  var path = document.location.pathname;
  var query = document.location.search;
  var redirect = encodeURIComponent(path + query);
  var loginUrl = "/login?redirect_uri="+ redirect;
  document.location.href = loginUrl;
});


app.ports.tooltip.subscribe(function (pipelineInfo) {
  var pipelineName = pipelineInfo[0];
  var pipelineTeamName = pipelineInfo[1];

  var team = $('div[id="' + pipelineTeamName + '"]');
  var title = team.find('.card[data-pipeline-name="' + pipelineName + '"]').find('.dashboard-pipeline-name');

  if(title.get(0).offsetWidth < title.get(0).scrollWidth){
      title.parent().attr('data-tooltip', pipelineName);
  }
});

app.ports.tooltipHd.subscribe(function (pipelineInfo) {
  var pipelineName = pipelineInfo[0];
  var pipelineTeamName = pipelineInfo[1];

  var title = $('.card[data-pipeline-name="' + pipelineName + '"][data-team-name="' + pipelineTeamName + '"]').find('.dashboardhd-pipeline-name');

  if(title.get(0).offsetWidth < title.get(0).scrollWidth){
      title.parent().attr('data-tooltip', pipelineName);
  }
});

var storageKey = "csrf_token";
app.ports.saveToken.subscribe(function(value) {
  localStorage.setItem(storageKey, value);
});
app.ports.loadToken.subscribe(function() {
  app.ports.tokenReceived.send(localStorage.getItem(storageKey));
});
window.addEventListener('storage', function(event) {
  if (event.key == storageKey) {
    app.ports.tokenReceived.send(localStorage.getItem(storageKey));
  }
}, false);

var sideBarStateKey = "is_sidebar_open";
app.ports.loadSideBarState.subscribe(function() {
  var sideBarState = sessionStorage.getItem(sideBarStateKey);
  app.ports.sideBarStateReceived.send(sideBarState);
});
app.ports.saveSideBarState.subscribe(function(isOpen) {
  sessionStorage.setItem(sideBarStateKey, isOpen);
});

app.ports.openEventStream.subscribe(function(config) {
  var buffer = [];
  var es = new EventSource(config.url);
  function flush() {
    if (buffer.length > 0) {
      app.ports.eventSource.send(buffer);
      buffer = [];
    }
  }
  function dispatchEvent(event) {
    buffer.push(event);
    if (buffer.length > 1000) {
      flush();
    }
  }
  es.onopen = dispatchEvent;
  es.onerror = dispatchEvent;
  config.eventTypes.forEach(function(eventType) {
    es.addEventListener(eventType, dispatchEvent);
  });
  app.ports.closeEventStream.subscribe(function() {
    es.close();
  });
  setInterval(flush, 200);
});

app.ports.checkIsVisible.subscribe(function(id) {
  var interval = setInterval(function() {
    var element = document.getElementById(id);
    if (element) {
      clearInterval(interval);
      var isVisible =
        element.getBoundingClientRect().left < window.innerWidth;
      app.ports.reportIsVisible.send([id, isVisible]);
    }
  }, 20);
});

app.ports.setFavicon.subscribe(function(url) {
  var oldIcon = document.getElementById("favicon");
  var newIcon = document.createElement("link");
  newIcon.id = "favicon";
  newIcon.rel = "shortcut icon";
  newIcon.href = url;
  if (oldIcon) {
    document.head.removeChild(oldIcon);
  }

  document.head.appendChild(newIcon);
});

app.ports.rawHttpRequest.subscribe(function(url) {
  var xhr = new XMLHttpRequest();

  xhr.addEventListener('error', function(error) {
    app.ports.rawHttpResponse.send('networkError');
  });
  xhr.addEventListener('timeout', function() {
    app.ports.rawHttpResponse.send('timeout');
  });
  xhr.addEventListener('load', function() {
    app.ports.rawHttpResponse.send('success');
  });

  xhr.open('GET', url, false);

  try {
    xhr.send();
    if (xhr.readyState === 1) {
      app.ports.rawHttpResponse.send('browserError');
    }
  } catch (error) {
    app.ports.rawHttpResponse.send('networkError');
  }
});

app.ports.renderSvgIcon.subscribe(function(icon, id) {
  addIcon(icon, (typeof id !== 'undefined') ? id : icon);
});

var clipboard = new ClipboardJS('#copy-token');
