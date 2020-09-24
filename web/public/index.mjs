import "./d3.v355.min.js";
import { Graph, GraphNode } from './graph.mjs';

const iconsModulePromise = import("./mdi-svg.min.js");

export function renderPipeline(jobs, resources, newUrl){
  const foundSvg = d3.select(".pipeline-graph");
  const svg = createPipelineSvg(foundSvg)
  if (svg.node() != null) {
    draw(svg, jobs, resources, newUrl);
  }
}

var currentHighlight;

var redraw;
function draw(svg, jobs, resources, newUrl) {
  redraw = redrawFunction(svg, jobs, resources, newUrl);
  redraw();
}

function redrawFunction(svg, jobs, resources, newUrl) {
  return function() {
    // reset viewbox so calculations are done from a blank slate.
    //
    // without this text and boxes jump around on every redraw,
    // in affected browsers (seemingly anything but Chrome + OS X).
    d3.select(svg.node().parentNode).attr("viewBox", "0 0 0 0");

    var graph = createGraph(svg, jobs, resources);

    svg.selectAll("g.edge").remove();
    svg.selectAll("g.node").remove();

    var svgEdges = svg.selectAll("g.edge")
      .data(graph.edges());

    svgEdges.exit().remove();

    var svgNodes = svg.selectAll("g.node")
      .data(graph.nodes());

    svgNodes.exit().remove();

    var svgEdge = svgEdges.enter().append("g")
      .attr("class", function(edge) { return "edge " + edge.source.node.status })

    svgEdges.each(function(edge) {
      if (edge.customData !== null && edge.customData.trigger === false) {
        d3.select(this).classed("trigger-false", true)
      }
    })

    function highlight(thing) {
      if (!thing.key) {
        return
      }

      currentHighlight = thing.key;

      svgEdges.each(function(edge) {
        if (edge.source.key == thing.key) {
          d3.select(this).classed({
            active: true
          })
        }
      })

      svgNodes.each(function(node) {
        if (node.key == thing.key) {
          d3.select(this).classed({
            active: true
          })
        }
      })
    }

    function lowlight(thing) {
      if (!thing.key) {
        return
      }

      currentHighlight = undefined;

      svgEdges.classed({ active: false })
      svgNodes.classed({ active: false })
    }

    var svgNode = svgNodes.enter().append("g")
      .attr("class", function(node) { return "node " + node.class })
      .on("mouseover", highlight)
      .on("mouseout", lowlight)

    var nodeLink = svgNode.append("svg:a")
      .attr("xlink:href", function(node) { return node.url })
      .on("click", function(node) {
        var ev = d3.event;
        if (ev.defaultPrevented) return; // dragged

        if (ev.ctrlKey || ev.altKey || ev.metaKey || ev.shiftKey) {
          return;
        }

        if (ev.button != 0) {
          return;
        }

        ev.preventDefault();

        newUrl.send(node.url);
      })

    var jobStatusBackground = nodeLink.append("rect")
      .attr("height", function(node) { return node.height() })


    var animatableBackground = nodeLink.append("foreignObject")
      .attr("class", "js-animation-wrapper")
      .attr("height", function(node) { return node.height() + (2 * node.animationRadius()) })
      .attr("x", function(node) { return -node.animationRadius()})
      .attr("y", function(node) { return -node.animationRadius()})

    var animationPadding = animatableBackground.append("xhtml:div")
      .style("padding", function(node) {
        return node.animationRadius() + "px";
      })

    animationPadding.style("height", function(node) { return node.height() + "px" })

    var animationTarget = animationPadding.append("xhtml:div")

    animationTarget.attr("class", "animation")
    animationTarget.style("height", function(node) { return node.height() + "px" })

    var pinIconWidth = 6;
    var pinIconHeight = 9.75;
    nodeLink.filter(function(node) { return node.pinned() }).append("image")
        .attr("xlink:href", "/public/images/pin-ic-white.svg")
        .attr("width", pinIconWidth)
        .attr("y", function(node) { return node.height() / 2 - pinIconHeight / 2 })
        .attr("x", function(node) { return node.padding() })

    var iconSize = 12;
    nodeLink.filter(function(node) { return node.has_icon() }).append("use")
      .attr("xlink:href", function(node) { return "#" + node.id + "-svg-icon" })
      .attr("width", iconSize)
      .attr("height", iconSize)
      .attr("fill", "white")
      .attr("y", function(node) { return node.height() / 2 - iconSize / 2 })
      .attr("x", function(node) { return node.padding() + (node.pinned() ? pinIconWidth + node.padding() : 0) })

    nodeLink.append("text")
      .text(function(node) { return node.name })
      .attr("dominant-baseline", "middle")
      .attr("text-anchor", function(node) { return node.pinned() || node.has_icon() ? "end" : "middle" })
      .attr("x", function(node) { return node.pinned() || node.has_icon() ? node.width() - node.padding() : node.width() / 2 })
      .attr("y", function(node) { return node.height() / 2 })

    jobStatusBackground.attr("width", function(node) { return node.width() })
    animatableBackground.attr("width", function(node) { return node.width() + (2 * node.animationRadius()) })
    animationTarget.style("width", function(node) { return node.width() + "px" })
    animationPadding.style("width", function(node) { return node.width() + "px" })

    graph.layout()

    var failureCenters = []
    var epsilon = 2
    var graphNodes = graph.nodes()
    for (var i in graphNodes) {
      if (graphNodes[i].status == "failed") {
        var xCenter = graphNodes[i].position().x + (graphNodes[i].width() / 2)
        var found = false
        for (var i in failureCenters) {
          if (Math.abs(xCenter - failureCenters[i]) < epsilon) {
            found = true
            break
          }
        }
        if(!found) {
          failureCenters.push(xCenter)
        }
      }
    }

    svg.selectAll("g.fail-triangle-node").remove()
    const failTriangleBottom = 20
    const failTriangleHeight = 24
    for (var i in failureCenters) {
      var triangleNode = svg.append("g")
        .attr("class", "fail-triangle-node")
      var triangleOutline = triangleNode.append("path")
        .attr("class", "fail-triangle-outline")
        .attr("d", "M191.62,136.3778H179.7521a5,5,0,0,1-4.3309-7.4986l5.9337-10.2851a5,5,0,0,1,8.6619,0l5.9337,10.2851A5,5,0,0,1,191.62,136.3778Z")
        .attr("transform", "translate(-174.7446 -116.0927)")
      var triangle = triangleNode.append("path")
        .attr("class", "fail-triangle")
        .attr("d", "M191.4538,133.0821H179.9179a2,2,0,0,1-1.7324-2.9994l5.7679-9.9978a2,2,0,0,1,3.4647,0l5.7679,9.9978A2,2,0,0,1,191.4538,133.0821Z")
        .attr("transform", "translate(-174.7446 -116.0927)")
      var triangleBBox = triangleNode.node().getBBox()
      var triangleScale = failTriangleHeight / triangleBBox.height
      var triangleWidth = triangleBBox.width * triangleScale
      var triangleX = failureCenters[i] - (triangleWidth / 2)
      var triangleY = -failTriangleBottom - failTriangleHeight
      triangleNode.attr("transform", "translate(" + triangleX + ", " + triangleY + ") scale(" + triangleScale + ")")
    }

    nodeLink.attr("class", function(node) {
      var classes = [];

      if (node.debugMarked) {
        classes.push("marked");
      }

      if (node.columnMarked) {
        classes.push("column-marked");
      }

      return classes.join(" ");
    });

    svgNode.attr("transform", function(node) {
      var position = node.position();
      return "translate("+position.x+", "+position.y+")"
    })

    svgEdge.append("path")
      .attr("d", function(edge) { return edge.path() })
      .on("mouseover", highlight)
      .on("mouseout", lowlight)

    var bbox = svg.node().getBBox();
    d3.select(svg.node().parentNode)
      .attr("viewBox", "" + (bbox.x - 20) + " " + (bbox.y - 20) + " " + (bbox.width + 40) + " " + (bbox.height + 40))

    const originalJobs = [...document.querySelectorAll(".job")];
    const jobAnimations = originalJobs.map(el => el.cloneNode(true));
    jobAnimations.forEach(el => {
      const foreignObject = el.querySelector('foreignObject');
      if (foreignObject != null) {
        removeElement(foreignObject);
      }
      el.classList.remove('job');
      el.classList.add('job-animation-node');
      el.querySelectorAll('a').forEach(removeElement);
      if (foreignObject != null) {
        removeElement(foreignObject);
      }
      el.appendChild(foreignObject);

      el.querySelectorAll('text').forEach(removeElement);
    });
    originalJobs.forEach(el => 
      el.querySelectorAll('.js-animation-wrapper').forEach(removeElement)
    );
    const canvas = document.querySelector('svg > g');
    if (canvas != null) {
      canvas.prepend(...jobAnimations);
    }

    const largestEdge = Math.max(bbox.width, bbox.height);
    const animations = document.querySelectorAll('.animation');
    animations.forEach(el => {
      if (largestEdge < 500) {
        el.classList.add("animation-small");
      } else if (largestEdge < 1500) {
        el.classList.add("animation-medium");
      } else if (largestEdge < 3000) {
        el.classList.add("animation-large");
      } else {
        el.classList.add("animation-xlarge");
      }
    })

    if (currentHighlight) {
      svgNodes.each(function(node) {
        if (node.key == currentHighlight) {
          highlight(node)
        }
      });

      svgEdges.each(function(node) {
        if (node.key == currentHighlight) {
          highlight(node)
        }
      });
    }
  }
};

function removeElement(el) {
  if (el == null) {
    return;
  }
  if (el.parentNode == null) {
    return;
  }
  el.parentNode.removeChild(el);
}

var zoom = (function() {
  var z;
  return function() {
    z = z || d3.behavior.zoom();
    return z;
  }
})();

var shouldResetPipelineFocus = false;

function createPipelineSvg(svg) {
  var g = d3.select("g.test")
  if (g.empty()) {
    svg.append("defs").append("filter")
      .attr("id", "embiggen")
      .append("feMorphology")
      .attr("operator", "dilate")
      .attr("radius", "4");

    g = svg.append("g").attr("class", "test")
    svg.on("mousedown", function() {
      var ev = d3.event;
      if (ev.button || ev.ctrlKey)
        ev.stopImmediatePropagation();
      }).call(zoom().scaleExtent([0.5, 10]).on("zoom", function() {
      var ev = d3.event;
      if (shouldResetPipelineFocus) {
        shouldResetPipelineFocus = false;
        resetPipelineFocus();
      } else {
        g.attr("transform", "translate(" + ev.translate + ") scale(" + ev.scale + ")");
      }
    }));
  }
  return g
}

export function resetPipelineFocus() {
  var g = d3.select("g.test");

  if (!g.empty()) {
    g.attr("transform", "");
    zoom().translate([0,0]).scale(1).center(0,0);
  } else {
    shouldResetPipelineFocus = true
  }

  return g
}

function createGraph(svg, jobs, resources) {
  var graph = new Graph();

  var resourceURLs = {};
  var resourceFailing = {};
  var resourcePinned = {};
  var resourceIcons = {};

  for (var i in resources) {
    var resource = resources[i];
    resourceURLs[resource.name] = "/pipelines/"+resource.pipeline_id+"/resources/"+encodeURIComponent(resource.name);
    resourceFailing[resource.name] = resource.failing_to_check;
    resourcePinned[resource.name] = resource.pinned_version;
    resourceIcons[resource.name] = resource.icon;
  }

  for (var i in jobs) {
    var job = jobs[i];

    var id = jobNode(job.name);

    var classes = ["job"];

    var url = "/pipelines/"+job.pipeline_id+"/jobs/"+encodeURIComponent(job.name);
    if (job.next_build) {
      var build = job.next_build
      url = "/pipelines/"+build.pipeline_id+"/jobs/"+encodeURIComponent(build.job_name)+"/builds/"+build.name;
    } else if (job.finished_build) {
      var build = job.finished_build
      url = "/pipelines/"+build.pipeline_id+"/jobs/"+encodeURIComponent(build.job_name)+"/builds/"+build.name;
    }

    var status;
    if (job.paused) {
      status = "paused";
    } else if (job.finished_build) {
      status = job.finished_build.status
    } else {
      status = "no-builds";
    }

    classes.push(status);

    if (job.next_build) {
      classes.push(job.next_build.status);
    }

    graph.setNode(id, new GraphNode({
      id: id,
      name: job.name,
      class: classes.join(" "),
      status: status,
      url: url,
      svg: svg,
    }));
  }

  var resourceStatus = function (resource) {
    var status = "";
    if (resourceFailing[resource]) {
      status += " failing";
    }
    if (resourcePinned[resource]) {
      status += " pinned";
    }

    return status;
  };

  // populate job output nodes and edges
  for (var i in jobs) {
    var job = jobs[i];
    var id = jobNode(job.name);

    for (var j in job.outputs) {
      var output = job.outputs[j];

      var outputId = outputNode(job.name, output.resource);

      var jobOutputNode = graph.node(outputId);
      if (!jobOutputNode) {
        addIcon(resourceIcons[output.resource], outputId);
        jobOutputNode = new GraphNode({
          id: outputId,
          name: output.resource,
          icon: resourceIcons[output.resource],
          key: output.resource,
          class: "output" + resourceStatus(output.resource),
          repeatable: true,
          url: resourceURLs[output.resource],
          svg: svg
        });

        graph.setNode(outputId, jobOutputNode);
      }

      graph.addEdge(id, outputId, output.resource, null)
    }
  }

  // populate dependant job input edges
  //
  // do this first as this is what primarily determines node ranks
  for (var i in jobs) {
    var job = jobs[i];
    var id = jobNode(job.name);

    for (var j in job.inputs) {
      var input = job.inputs[j];

      if (input.passed && input.passed.length > 0) {
        for (var p in input.passed) {
          var sourceJobNode = jobNode(input.passed[p]);

          var sourceOutputNode = outputNode(input.passed[p], input.resource);
          var sourceInputNode = inputNode(input.passed[p], input.resource);

          var sourceNode;
          if (graph.node(sourceOutputNode)) {
            sourceNode = sourceOutputNode;
          } else {
            if (!graph.node(sourceInputNode)) {
              addIcon(resourceIcons[input.resource], sourceInputNode);
              graph.setNode(sourceInputNode, new GraphNode({
                id: sourceInputNode,
                name: input.resource,
                icon: resourceIcons[input.resource],
                key: input.resource,
                class: "constrained-input" + (resourcePinned[input.resource] ? " pinned" : ""),
                repeatable: true,
                url: resourceURLs[input.resource],
                svg: svg
              }));
            }

            if (graph.node(sourceJobNode)) {
              graph.addEdge(sourceJobNode, sourceInputNode, input.resource, null);
            }

            sourceNode = sourceInputNode;
          }

          graph.addEdge(sourceNode, id, input.resource, {trigger: input.trigger});
        }
      }
    }
  }

  // populate unconstrained job inputs
  //
  // now that we know the rank, draw one unconstrained input per rank
  for (var i in jobs) {
    var job = jobs[i];
    var id = jobNode(job.name);

    var node = graph.node(id);

    for (var j in job.inputs) {
      var input = job.inputs[j];
      var status = "";

      if (!input.passed || input.passed.length == 0) {
        var inputId = inputNode(job.name, input.resource+"-unconstrained");

        if (!graph.node(inputId)) {
          addIcon(resourceIcons[input.resource], inputId);
          graph.setNode(inputId, new GraphNode({
            id: inputId,
            name: input.resource,
            icon: resourceIcons[input.resource],
            key: input.resource,
            class: "input" + resourceStatus(input.resource),
            status: status,
            repeatable: true,
            url: resourceURLs[input.resource],
            svg: svg,
            equivalentBy: input.resource+"-unconstrained",
          }));
        }

        graph.addEdge(inputId, id, input.resource, {trigger: input.trigger})
      }
    }
  }

  graph.computeRanks();
  graph.collapseEquivalentNodes();
  graph.addSpacingNodes();

  return graph;
}

export function addIcon(iconName, nodeId) {
  iconsModulePromise.then(icons => {
    var id = nodeId + "-svg-icon";
    if (document.getElementById(id) === null) {
      var svg = icons.svg(iconName, id);
      var template = document.createElement('template');
      template.innerHTML = svg;
      var icon = template.content.firstChild;
      var iconStore = document.getElementById("icon-store");
      if (iconStore == null) {
        iconStore = createIconStore();
      }
      iconStore.appendChild(icon)
    }
  })
}

function createIconStore() {
  const iconStore = document.createElement('div');
  iconStore.id = "icon-store";
  iconStore.style.display = "none";
  document.body.appendChild(iconStore);
  return iconStore;
}

function groupNode(name) {
  return "group-"+name;
}

function resourceNode(name) {
  return "resource-"+name;
}

function jobNode(name) {
  return "job-"+name;
}

function gatewayNode(jobNames) {
  return "gateway-"+jobNames.sort().join("-");
}

function outputNode(jobName, resourceName) {
  return "job-"+jobName+"-output-"+resourceName;
}

function inputNode(jobName, resourceName) {
  return "job-"+jobName+"-input-"+resourceName;
}
