import "./d3.v355.min.js";
import './graphviz.min.js';

export function renderCausality(dot){
  const foundSvg = d3.select(".causality-graph");
  const svg = createSvg(foundSvg);

  graphviz.graphviz.layout(dot, "svg", "dot").then(content => {
    svg.html(content);

    // disable tooltips that graphviz auto-generates
    svg.selectAll('title').remove();
    svg.selectAll('*').attr('xlink:title', null);

    var bbox = svg.node().getBBox()
    d3.select(svg.node().parentNode)
      .attr("viewBox", "" + (bbox.x - 20) + " " + (bbox.y - 20) + " " + (bbox.width + 40) + " " + (bbox.height + 40))
  })
}

function createSvg(svg) {
  var g = d3.select("g.test")
  var zoom = d3.behavior.zoom();

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
      }
    )
    .call(zoom
      .scaleExtent([0.5, 10])
      .on("zoom", function() {
        var ev = d3.event;
        g.attr("transform", "translate(" + ev.translate + ") scale(" + ev.scale + ")");
      })
    );
  }
  return g;
}
