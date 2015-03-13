var Immutable = require("immutable");

function OrderedTree() {
  this.tree = Immutable.List();

  this.add = function(location, value) {
    for (var i = 0; i < location.length; i++) {
      this.tree = this.tree.updateIn(location.slice(0, i+1), function(ele) {
        return ele || Immutable.List();
      });
    }

    this.tree = this.tree.setIn(location, value);
  }

  this.walk = function(cb) {
    walk(this.tree, function(x) {
      if (x !== undefined) {
        return cb(x);
      }
    });
  }
}

function walk(iterable, cb) {
  iterable.forEach(function(x) {
    if (Immutable.Iterable.isIterable(x)) {
      walk(x, cb);
    } else {
      return cb(x);
    }
  })
}

module.exports.OrderedTree = OrderedTree;
module.exports.walk = walk;
