import "./d3.v355.min.js";
import "./dagre-d3.v064.min.js"
import "./graphlib-dot.v064.min.js"

export function renderCausality(dot){
  const foundSvg = d3.select(".causality-graph");
  const svg = createSvg(foundSvg)

  var render = dagreD3.render();
  var g = graphlibDot.read(dot);

  render(svg, g)

  var bbox = svg.node().getBBox()
  d3.select(svg.node().parentNode)
    .attr("viewBox", "" + (bbox.x - 20) + " " + (bbox.y - 20) + " " + (bbox.width + 40) + " " + (bbox.height + 40))
}

function resize(svg) {
}

function createSvg(svg) {
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
      g.attr("transform", "translate(" + ev.translate + ") scale(" + ev.scale + ")");
    }));
  }
  return g
}

var zoom = (function() {
  var z;
  return function() {
    z = z || d3.behavior.zoom();
    return z;
  }
})();

