function draw(groups, renderFn) {
  $.get("/api/v1/jobs", function(jobsPayload) {
    var jobs = JSON.parse(jobsPayload);

    $.get("/api/v1/resources", function(resourcesPayload) {
      var resources = JSON.parse(resourcesPayload);

      renderFn(jobs, resources);
    });
  });
}

var currentHighlight;

function drawContinuously(svg, groups) {
  draw(groups, function(jobs, resources) {
    var graph = createGraph(svg, groups, jobs, resources);

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

    var nodeBackground = nodeLink.append("rect")
      .attr("height", function(node) { return node.height() })

    nodeLink.append("text")
      .text(function(node) { return node.name })
      .attr("dominant-baseline", "middle")
      .attr("text-anchor", "middle")
      .attr("x", function(node) { return node.width() / 2 })
      .attr("y", function(node) { return node.height() / 2 })

    nodeBackground.attr("width", function(node) { return node.width() })

    graph.layout()

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

    setTimeout(function() {
      drawContinuously(svg, groups)
    }, 4000)
  });
}

function renderPipeline(groups) {
  var svg = d3.select("#pipeline")
    .append("svg")
      .attr("width", "100%")
      .attr("height", "100%");

  svg.append("defs").append("filter")
    .attr("id", "embiggen")
    .append("feMorphology")
    .attr("operator", "dilate")
    .attr("radius", "4")

  var g = svg.append("g");
  svg.call(d3.behavior.zoom().on("zoom", function() {
    var ev = d3.event;
    g.attr("transform", "translate(" + ev.translate + ") scale(" + ev.scale + ")");
  }));

  drawContinuously(g, groups);

  $("ul.groups li a").click(function(e) {
    var group = e.target.text;

    if (e.shiftKey) {
      groups[group] = !groups[group];
    } else {
      for (var name in groups) {
        groups[name] = name == group;
      }
    }

    var groupQueries = [];
    for (var name in groups) {
      if (groups[name]) {
        groupQueries.push("groups="+name);
      }
    }

    window.location.search = "?" + groupQueries.join("&");

    return false;
  });
}

function createGraph(svg, groups, jobs, resources) {
  var graph = new Graph();

  var resourceURLs = {};

  for (var i in resources) {
    var resource = resources[i];
    resourceURLs[resource.name] = resource.url;
  }

  for (var i in jobs) {
    var job = jobs[i];

    if (!groupsMatch(job.groups, groups)) {
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
    if (job.finished_build) {
      status = job.finished_build.status
    } else {
      status = "pending";
    }

    classes.push(status);

    if (job.next_build) {
      classes.push("started");
    }

    graph.setNode(id, new Node({
      id: id,
      name: job.name,
      class: classes.join(" "),
      status: status,
      url: url,
      svg: svg
    }));
  }

  // populate job output nodes and edges
  for (var i in jobs) {
    var job = jobs[i];
    var id = jobNode(job.name);

    if (!groupsMatch(job.groups, groups)) {
      continue;
    }

    for (var j in job.outputs) {
      var output = job.outputs[j];

      var outputId = outputNode(job.name, output.resource);

      graph.setNode(outputId, new Node({
        id: outputId,
        name: output.resource,
        key: output.resource,
        class: "output",
        url: resourceURLs[output.resource],
        svg: svg
      }));

      graph.addEdge(id, outputId, output.resource)
    }
  }

  // populate dependant job input edges
  //
  // do this first as this is what primarily determines node ranks
  for (var i in jobs) {
    var job = jobs[i];
    var id = jobNode(job.name);

    if (!groupsMatch(job.groups, groups)) {
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
                url: resourceURLs[input.resource],
                svg: svg
              }));
            }

            if (graph.node(sourceJobNode)) {
              graph.addEdge(sourceJobNode, sourceInputNode, input.resource);
            }

            sourceNode = sourceInputNode;
          }

          graph.addEdge(sourceNode, id, input.resource);
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

    if (!groupsMatch(job.groups, groups)) {
      continue;
    }

    var node = graph.node(id);
    var rank = node.rank();

    for (var j in job.inputs) {
      var input = job.inputs[j];

      if (!input.passed || input.passed.length == 0) {
        var inputId = inputNode(rank, input.resource);

        if (!graph.node(inputId)) {
          graph.setNode(inputId, new Node({
            id: inputId,
            name: input.resource,
            key: input.resource,
            class: "input",
            url: resourceURLs[input.resource],
            svg: svg
          }));
        }

        graph.addEdge(inputId, id, input.resource)
      }
    }
  }

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
  return "gateway-"+jobNames.sort().join("-")
}

function outputNode(jobName, resourceName) {
  return "job-"+jobName+"-output-"+resourceName
}

function inputNode(jobName, resourceName) {
  return "job-"+jobName+"-input-"+resourceName
}
