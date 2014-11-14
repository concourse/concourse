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

      cGraph.setEdge(id, destinationNode, {
        "id": "job-output-"+id+"-to-"+destinationNode,
        "status": status,
        "arrowhead": "status",
      });
    }
  }
}

function edgeFromResource(graph, resourceName, destinationNode) {
  var sourceNode = resourceNode(resourceName);

  graph.setEdge(sourceNode, destinationNode, {
    "id": "resource-input-"+sourceNode+"-to-"+destinationNode,
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
      "id": "job-input-"+sourceNode+"-to-"+destinationNode,
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

function inlineNodesIntoCommonGroup(cGraph) {
  cGraph.nodes().forEach(function(v) {
    var commonGroup;

    var outE = cGraph.outEdges(v);
    for(var o in outE) {
      var edge = outE[o];
      var parent = cGraph.parent(edge.w);

      if (!commonGroup) {
        commonGroup = parent;
      }

      if (commonGroup != parent) {
        return
      }
    }

    var inE = cGraph.inEdges(v);
    for(var i in inE) {
      var edge = inE[i];
      var parent = cGraph.parent(edge.v);

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
  for (var group in groups) {
    var enabled = groups[group];
    if (enabled) {
      continue;
    }

    var id = groupNode(group);

    digraph.children(id).forEach(function(v) {
      var outE = digraph.outEdges(v);
      for(var o in outE) {
        var edge = outE[o];

        var targetValue = digraph.node(edge.w);
        if (nodeIsInGroups(groups, targetValue)) {
          return;
        }
      }

      var inE = digraph.inEdges(v);
      for(var i in inE) {
        var edge = inE[i];

        var sourceValue = digraph.node(edge.v);
        if (nodeIsInGroups(groups, sourceValue)) {
          return;
        }
      }

      digraph.removeNode(v);
    });
  }
}

function removeOrphanedNodes(digraph) {
  digraph.nodes().forEach(function(v) {
    if (dagreD3.util.isSubgraph(digraph, v)) {
      return;
    }

    if (digraph.parent(v)) {
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
