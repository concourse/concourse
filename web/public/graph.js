function objectIsEmpty(obj) {
  for(var x in obj)
    return false;

  return true;
}

function generateGraph(groups, jobs, resources) {
  var cGraph = new dagreD3.graphlib.Graph({
    compound: true
  }).setGraph({
    rankDir: "LR"
  });

  populateGroupNodes(cGraph, groups);
  populateResourceNodes(cGraph, resources);
  populateJobNodesAndEdges(cGraph, jobs);
  inlineNodesIntoCommonGroup(cGraph);

  if (!objectIsEmpty(groups)) {
    removeUnconnectedGroupMembers(groups, cGraph);
    removeOrphanedNodes(cGraph);
  }

  return cGraph;
}

function populateGroupNodes(cGraph, groups) {
  for (var name in groups) {
    cGraph.setNode(groupNode(name), {});
  }
}

function populateResourceNodes(cGraph, resources) {
  for (var i in resources) {
    var resource = resources[i];
    var id = resourceNode(resource.name);

    var classes = ["resource"];

    if (resource.failing) {
      classes.push("failing");
    }

    if (resource.checking) {
      classes.push("checking")
    }

    cGraph.setNode(id, {
      resource: resource.name,
      class: classes.join(" "),
      label: "<h1 class=\"resource\"><a href=\"" + resource.url + "\">" + resource.name + "</a></h1>",
      labelType: "html",
      groups: resource.groups
    });

    if (resource.groups.length == 1) {
      cGraph.setParent(id, groupNode(resource.groups[0]));
    }
  }
}

function populateJobNodesAndEdges(cGraph, jobs) {
  // populate all job nodes first, so that they can be interconnected
  for (var i in jobs) {
    var job = jobs[i];
    var id = jobNode(job.name);

    var classes = ["job"];

    var status = "normal";
    var url = job.url;
    if (job.next_build) {
      status = job.next_build.status;
      url = job.next_build.url;
    } else if (job.finished_build) {
      status = job.finished_build.status;
      url = job.finished_build.url;
    }

    classes.push(status);

    cGraph.setNode(id, {
      job: job.name,
      class: classes.join(" "),
      status: status,
      label: "<h1 class=\"job\"><a href=\"" + url + "\">" + job.name + "</a></h1>",
      labelType: "html",
      groups: job.groups,
      totalInputs: job.inputs.length
    });

    if (job.groups.length == 1) {
      cGraph.setParent(id, groupNode(job.groups[0]));
    }
  }

  // populate job input and output edges
  for (var i in jobs) {
    var job = jobs[i];
    var id = jobNode(job.name);

    for (var j in job.inputs) {
      var input = job.inputs[j];

      if (input.hidden) {
        continue;
      }

      if (!input.passed || input.passed.length == 0) {
        edgeFromResource(cGraph, input.resource, id);
      } else if (input.passed.length == 1) {
        edgeFromJob(cGraph, input.passed[0], id, input.resource);
      } else {
        edgeFromGateway(cGraph, input.passed, id, input.resource);
      }
    }

    for (var j in job.outputs) {
      var output = job.outputs[j];
      var destinationNode = resourceNode(output.resource);

      edgeFromJob(cGraph, job.name, destinationNode);
    }
  }
}

function edgeFromResource(graph, resourceName, destinationNode) {
  var sourceNode = resourceNode(resourceName);

  graph.setEdge(sourceNode, destinationNode, {
    "id": "resource-"+sourceNode+"-to-"+destinationNode,
    "status": "normal",
    "arrowhead": "status",
  });
}

function edgeFromJob(graph, sourceJobName, destinationNode, resourceName) {
  var sourceNode = jobNode(sourceJobName);
  var sourceJob = graph.node(sourceNode);

  var existingEdge = graph.edge(sourceNode, destinationNode);
  if (existingEdge && resourceName && sourceJob.totalInputs > 1) {
    existingEdge.label += "\n" + resourceName
  } else {
    var label;
    if (sourceJob.totalInputs > 1) {
      label = resourceName;
    }

    graph.setEdge(sourceNode, destinationNode, {
      "id": "job-"+sourceNode+"-to-"+destinationNode,
      "label": label,
      "status": sourceJob.status,
      "arrowhead": "status",
    });
  }
}

function edgeFromGateway(graph, gatewayJobNames, destinationNode, label) {
  var sourceNode = gatewayNode(gatewayJobNames);

  graph.setNode(sourceNode, {
    label: "",
    gateway: true,
    class: "gateway"
  });

  for (var i in gatewayJobNames) {
    edgeFromJob(graph, gatewayJobNames[i], sourceNode, label);
  }

  graph.setEdge(sourceNode, destinationNode, {
    "id": "gateway-"+sourceNode+"-to-"+destinationNode,
    "status": "normal",
    "arrowhead": "status"
  });
}

function _nodes(graph, v, edgesFn, edgeAttr) {
  var nodes = {};

  var edges = graph[edgesFn](v);
  for (var i in edges) {
    var edge = edges[i];
    var edgePoint = edge[edgeAttr];

    var node = graph.node(edgePoint);
    if (node.gateway) {
      var gatewayNodes = _nodes(graph, edgePoint, edgesFn, edgeAttr);
      for (var n in gatewayNodes) {
        nodes[n] = gatewayNodes[n]
      }
    } else {
      nodes[edgePoint] = node
    }
  }

  return nodes;
}

function upstreamNodes(graph, v) {
  return _nodes(graph, v, "inEdges", "v");
}

function downstreamNodes(graph, v) {
  return _nodes(graph, v, "outEdges", "w");
}

function inlineNodesIntoCommonGroup(cGraph) {
  cGraph.nodes().forEach(function(v) {
    var commonGroup;

    var outNodes = downstreamNodes(cGraph, v);
    for(var o in outNodes) {
      var parent = cGraph.parent(o);

      if (!commonGroup) {
        commonGroup = parent;
      }

      if (commonGroup != parent) {
        return
      }
    }

    var inNodes = upstreamNodes(cGraph, v);
    for(var i in inNodes) {
      var parent = cGraph.parent(i);

      if (!commonGroup) {
        commonGroup = parent;
      }

      if (commonGroup != parent) {
        return
      }
    }

    if (commonGroup) {
      cGraph.setParent(v, commonGroup);
    }
  });
}

function removeUnconnectedGroupMembers(groups, digraph) {
  digraph.nodes().forEach(function(v) {
    if (dagreD3.util.isSubgraph(digraph, v)) {
      return;
    }

    var value = digraph.node(v);
    if (value.gateway && connectedUpstreamAndDownstream(digraph, groups, v)) {
      return;
    }

    if (connectedUpstreamOrDownstream(digraph, groups, v)) {
      return;
    }

    if (!nodeIsInGroups(groups, value)) {
      digraph.removeNode(v);
    }
  });
}

function connectedUpstreamAndDownstream(digraph, groups, v) {
  var hasUpstreamNode, hasDownstreamNode;

  var inNodes = upstreamNodes(digraph, v);
  for(var i in inNodes) {
    if (nodeIsInGroups(groups, inNodes[i])) {
      hasUpstreamNode = true;
      break;
    }
  }

  var outNodes = downstreamNodes(digraph, v);
  for(var o in outNodes) {
    if (nodeIsInGroups(groups, outNodes[o])) {
      hasDownstreamNode = true;
      break;
    }
  }

  return hasUpstreamNode && hasDownstreamNode;
}

function connectedUpstreamOrDownstream(digraph, groups, v) {
  var value = digraph.node(v);

  var inNodes = upstreamNodes(digraph, v);
  for(var i in inNodes) {
    if (nodeIsInGroups(groups, inNodes[i])) {
      return true;
    }
  }

  var outNodes = downstreamNodes(digraph, v);
  for(var o in outNodes) {
    if (nodeIsInGroups(groups, outNodes[o])) {
      return true;
    }
  }

  return false;
}

function removeOrphanedNodes(digraph) {
  digraph.nodes().forEach(function(v) {
    if (dagreD3.util.isSubgraph(digraph, v)) {
      return;
    }

    if (!digraph.node(v).gateway && digraph.parent(v)) {
      return;
    }

    if (digraph.nodeEdges(v).length == 0) {
      digraph.removeNode(v);
    }
  });
}

function nodeIsInGroups(groups, value) {
  if (!value.groups) {
    return false;
  }

  for(var i in value.groups) {
    if (groups[value.groups[i]]) {
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
