const renderingModulePromise = import('./index.mjs');

const node = document.getElementById("elm-app-embed");
if (node === null) {
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
  if (team === null) {
    return;
  }
  const card = team.querySelector(`.card[data-pipeline-name="${pipelineName}"]`);
  if (card === null) {
    return;
  }
  let title = card.querySelector('.dashboard-pipeline-name');
  if (title === null) {
    title = card.querySelector('.dashboard-group-name');
  }

  if(title === null || title.offsetWidth >= title.scrollWidth) {
    return;
  }
  title.parentNode.setAttribute('data-tooltip', pipelineName);
});

app.ports.tooltipHd.subscribe(function (pipelineInfo) {
  var pipelineName = pipelineInfo[0];
  var pipelineTeamName = pipelineInfo[1];

  const card = document.querySelector(`.card[data-pipeline-name="${pipelineName}"][data-team-name="${pipelineTeamName}"]`);
  if (card === null) {
    return;
  }
  let title = card.querySelector('.dashboardhd-pipeline-name');
  if (title === null) {
    title = card.querySelector('.dashboardhd-group-name');
  }

  if(title === null || title.offsetWidth >= title.scrollWidth){
    return;
  }
  title.parentNode.setAttribute('data-tooltip', pipelineName);
});

app.ports.saveToLocalStorage.subscribe(function(params) {
  if (!params || params.length !== 2) {
    return;
  }
  const [key, value] = params;
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch(err) {
    console.error(err);
  }
});

app.ports.saveToSessionStorage.subscribe(function(params) {
  if (!params || params.length !== 2) {
    return;
  }
  const [key, value] = params;
  try {
    sessionStorage.setItem(key, JSON.stringify(value));
  } catch(err) {
    console.error(err);
  }
});

app.ports.loadFromLocalStorage.subscribe(function(key) {
  const value = localStorage.getItem(key);
  if (value === null) {
    return;
  }
  setTimeout(function() {
    app.ports.receivedFromLocalStorage.send([key, value]);
  }, 0);
});

app.ports.loadFromSessionStorage.subscribe(function(key) {
  const value = sessionStorage.getItem(key);
  if (value === null) {
    return;
  }
  setTimeout(function() {
    app.ports.receivedFromSessionStorage.send([key, value]);
  }, 0);
});

app.ports.deleteFromLocalStorage.subscribe(function(key) {
  localStorage.removeItem(key);
});


const csrfTokenKey = "csrf_token";
const favoritedPipelinesKey = "favorited_pipelines";
window.addEventListener('storage', function(event) {
  if (event.key === csrfTokenKey || event.key === favoritedPipelinesKey) {
    const value = localStorage.getItem(event.key);
    setTimeout(function() {
      app.ports.receivedFromLocalStorage.send([event.key, value]);
    }, 0);
  }
}, false);

app.ports.syncTextareaHeight.subscribe(function(id) {
  const attemptToSyncHeight = () => {
    const elem = document.getElementById(id);
    if (elem === null) {
      return false;
    }
    elem.style.height = "auto";
    elem.style.height = elem.scrollHeight + "px";
	return true;
  };
  setTimeout(() => {
    const success = attemptToSyncHeight();
    if (!success) {
	  // The element does not always exist by the time we attempt to sync
	  // Try one more time after a small delay
	  setTimeout(attemptToSyncHeight, 50);
	}
  }, 0);
});

let syncStickyBuildLogHeadersInterval;

app.ports.syncStickyBuildLogHeaders.subscribe(function() {
  if (!CSS || !CSS.supports || !CSS.supports('position', 'sticky')) {
    return;
  }
  if (syncStickyBuildLogHeadersInterval != null) {
    return;
  }
  const attemptToSync = () => {
    const padding = 5;
    const headers = document.querySelectorAll('.build-step .header:not(.loading-header)');
    if (headers.length === 0) {
      return false;
    }
    headers.forEach(header => {
      const parentHeader = findParentHeader(header);
      let curHeight = 0;
      if (parentHeader != null) {
        const parentHeight = parsePixels(parentHeader.style.top || '') || 0;
        curHeight = parentHeight + parentHeader.offsetHeight + padding;
      }
      header.style.top = curHeight + 'px';
    });
    return true;
  }

  setTimeout(() => {
    const success = attemptToSync();
    if (!success) {
      // The headers do not always exist by the time we attempt to sync.
      // Keep trying on an interval
      syncStickyBuildLogHeadersInterval = setInterval(() => {
        const success = attemptToSync();
        if (success) {
          clearInterval(syncStickyBuildLogHeadersInterval);
          syncStickyBuildLogHeadersInterval = null;
        }
      }, 250);
    }
  }, 50);
});

function findParentHeader(el) {
  const closestStepBody = el.closest('.step-body');
  if (closestStepBody == null || closestStepBody.parentElement == null) {
    return;
  }
  return closestStepBody.parentElement.querySelector('.header')
}

function parsePixels(raw) {
  raw = raw.trim();
  if(!raw.endsWith('px')) {
    return 0;
  }
  return parseFloat(raw);
}

app.ports.scrollToId.subscribe(function(params) {
  if (!params || params.length !== 2) {
    return;
  }
  const [parentId, toId] = params;
  const padding = 150;
  const interval = setInterval(function() {
    const parentElem = document.getElementById(parentId);
    if (parentElem === null) {
      return;
    }
    const elem = document.getElementById(toId);
    if (elem === null) {
      return;
    }
    parentElem.scrollTop = offsetTop(elem, parentElem) - padding;
    setTimeout(() => app.ports.scrolledToId.send([parentId, toId]), 50)
    clearInterval(interval);
  }, 20);
});

function offsetTop(element, untilElement) {
  let offsetTop = 0;
  while(element && element != untilElement) {
    offsetTop += element.offsetTop;
    element = element.offsetParent;
  }
  return offsetTop;
}

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
  renderingModulePromise.then(({addIcon}) => addIcon(icon, (typeof id !== 'undefined') ? id : icon));
});

var clipboard = new ClipboardJS('#copy-token');
