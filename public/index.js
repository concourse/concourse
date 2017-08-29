var currentHighlight;

function draw(svg, jobs, resources, newUrl) {
  concourse.redraw = redrawFunction(svg, jobs, resources, newUrl);
  concourse.redraw();
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

    nodeLink.append("text")
      .text(function(node) { return node.name })
      .attr("dominant-baseline", "middle")
      .attr("text-anchor", "middle")
      .attr("x", function(node) { return node.width() / 2 })
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
        xCenter = graphNodes[i].position().x + (graphNodes[i].width() / 2)
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
    failTriangleBottom = 20
    failTriangleHeight = 24
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

    var $jobs = $(".job")
    var jobAnimations = $jobs.clone();
    var largestEdge = Math.max(bbox.width, bbox.height);

    jobAnimations.each(function(i, el){
      var $el = $(el);
      var $foreignObject = $el.find('foreignObject').detach();
      $el.attr('class', $el.attr('class').replace('job', 'job-animation-node'));
      $el.find('a').remove();
      $el.append($foreignObject);
    });
    jobAnimations.find("text").remove();
    $jobs.find('.js-animation-wrapper').remove();
    $("svg > g").prepend(jobAnimations);


    if (largestEdge < 500) {
      $(".animation").addClass("animation-small");
    } else if (largestEdge < 1500) {
      $(".animation").addClass("animation-medium");
    } else if (largestEdge < 3000) {
      $(".animation").addClass("animation-large");
    } else {
      $(".animation").addClass("animation-xlarge");
    }

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

function resetPipelineFocus() {
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
  var resourcePaused = {};

  for (var i in resources) {
    var resource = resources[i];
    resourceURLs[resource.name] = resource.url;
    resourceFailing[resource.name] = resource.failing_to_check;
    resourcePaused[resource.name] = resource.paused;
  }

  for (var i in jobs) {
    var job = jobs[i];

    if (!groupsMatch(job.groups, concourse.groups)) {
      continue;
    }

    var id = jobNode(job.name);

    var classes = ["job"];

    var url = job.url;
    if (job.next_build) {
      url = job.next_build.url;
    } else if (job.finished_build) {
      url = job.finished_build.url;
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

    graph.setNode(id, new Node({
      id: id,
      name: job.name,
      class: classes.join(" "),
      status: status,
      url: url,
      svg: svg,
    }));
  }

  // populate job output nodes and edges
  for (var i in jobs) {
    var job = jobs[i];
    var id = jobNode(job.name);

    if (!groupsMatch(job.groups, concourse.groups)) {
      continue;
    }

    for (var j in job.outputs) {
      var output = job.outputs[j];

      var outputId = outputNode(job.name, output.resource);

      var jobOutputNode = graph.node(outputId);
      if (!jobOutputNode) {
        jobOutputNode = new Node({
          id: outputId,
          name: output.resource,
          key: output.resource,
          class: "output",
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

    if (!groupsMatch(job.groups, concourse.groups)) {
      continue;
    }

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
              graph.setNode(sourceInputNode, new Node({
                id: sourceInputNode,
                name: input.resource,
                key: input.resource,
                class: "constrained-input",
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

    if (!groupsMatch(job.groups, concourse.groups)) {
      continue;
    }

    var node = graph.node(id);

    for (var j in job.inputs) {
      var input = job.inputs[j];
      var status = "";

      if (!input.passed || input.passed.length == 0) {
        var inputId = inputNode(job.name, input.resource+"-unconstrained");

        if (!graph.node(inputId)) {
          var classes = "input";
          if (resourceFailing[input.resource]) {
            classes += " failing";
          }

          if (resourcePaused[input.resource]) {
            classes += " paused";
            status = "paused";
          }

          graph.setNode(inputId, new Node({
            id: inputId,
            name: input.resource,
            key: input.resource,
            class: classes,
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

function objectIsEmpty(o) {
  for (var x in o) {
    return false;
  }

  return true;
}

function groupsMatch(objGroups, groups) {
  if (objectIsEmpty(groups)) {
    return true;
  }

  for(var i in objGroups) {
    if (groups[objGroups[i]]) {
      return true;
    }
  }

  return false;
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
