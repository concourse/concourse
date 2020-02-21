const renderingModulePromise = import('./index.mjs');

const node = document.getElementById("elm-app-embed");
if (node == null) {
  throw "missing #elm-app-embed";
}

const app = Elm.Main.init({
  node: node,
  flags: window.elmFlags,
});

var resizeTimer;

app.ports.pinTeamNames.subscribe(function(config) {
  const sections = () => Array.from(document.querySelectorAll("." + config.sectionClass));
  const header = (section) => Array.from(section.childNodes).find(n => n.classList && n.classList.contains(config.sectionHeaderClass));
  const body = (section) => Array.from(section.childNodes).find(n => n.classList && n.classList.contains(config.sectionBodyClass));
  const lowestHeaderTop = (section) => body(section).offsetTop + body(section).scrollHeight - header(section).scrollHeight;

  const pageHeaderHeight = () => config.pageHeaderHeight;
  const viewportTop = () => window.pageYOffset + pageHeaderHeight();

  const updateHeader = (section) => {
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

  const updateSticky = () => {
    document.querySelector("." + config.pageBodyClass).style.marginTop = pageHeaderHeight();
    sections().forEach(updateHeader);
  }

  clearTimeout(resizeTimer);
  resizeTimer = setTimeout(updateSticky, 250);
  window.onscroll = updateSticky;
});

app.ports.resetPipelineFocus.subscribe(function() {
  renderingModulePromise.then(({resetPipelineFocus}) => resetPipelineFocus());
});

app.ports.renderPipeline.subscribe(function (values) {
  const jobs = values[0];
  const resources = values[1];
  renderingModulePromise.then(({renderPipeline}) =>
    // elm 0.17 bug, see https://github.com/elm-lang/core/issues/595
    setTimeout(() => renderPipeline(jobs, resources, app.ports.newUrl), 0)
  );
});

app.ports.requestLoginRedirect.subscribe(function (message) {
  var path = document.location.pathname;
  var query = document.location.search;
  var redirect = encodeURIComponent(path + query);
  var loginUrl = "/login?redirect_uri="+ redirect;
  document.location.href = loginUrl;
});


app.ports.tooltip.subscribe(function (pipelineInfo) {
  const pipelineName = pipelineInfo[0];
  const pipelineTeamName = pipelineInfo[1];

  const team = document.getElementById(pipelineTeamName);
  if (team == null) {
    return;
  }
  const card = team.querySelector(`.card[data-pipeline-name="${pipelineName}"]`);
  if (card == null) {
    return;
  }
  const title = card.querySelector('.dashboard-pipeline-name');
  if(title == null || title.offsetWidth >= title.scrollWidth) {
    return;
  }
  title.parentNode.setAttribute('data-tooltip', pipelineName);
});

app.ports.tooltipHd.subscribe(function (pipelineInfo) {
  var pipelineName = pipelineInfo[0];
  var pipelineTeamName = pipelineInfo[1];

  const card = document.querySelector(`.card[data-pipeline-name="${pipelineName}"][data-team-name="${pipelineTeamName}"]`);
  if (card == null) {
    return;
  }
  const title = card.querySelector('.dashboardhd-pipeline-name');

  if(title == null || title.offsetWidth >= title.scrollWidth){
    return;
  }
  title.parentNode.setAttribute('data-tooltip', pipelineName);
});

app.ports.saveToLocalStorage.subscribe(function(params) {
  if (!params || params.length !== 2) {
    return;
  }
  const [key, value] = params;
  localStorage.setItem(key, value);
});

app.ports.saveToSessionStorage.subscribe(function(params) {
  if (!params || params.length !== 2) {
    return;
  }
  const [key, value] = params;
  sessionStorage.setItem(key, value);
});

app.ports.loadFromLocalStorage.subscribe(function(key) {
  const value = localStorage.getItem(key);
  if (value === null) {
    return;
  }
  app.ports.receivedFromLocalStorage.send([key, value]);
});

app.ports.loadFromSessionStorage.subscribe(function(key) {
  const value = sessionStorage.getItem(key);
  if (value === null) {
    return;
  }
  app.ports.receivedFromSessionStorage.send([key, value]);
});

const csrfTokenKey = "csrf_token";
window.addEventListener('storage', function(event) {
  if (event.key === csrfTokenKey) {
    app.ports.receivedFromLocalStorage.send([csrfTokenKey, localStorage.getItem(csrfTokenKey)]);
  }
}, false);

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
