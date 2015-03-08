var Immutable = require("immutable");

function walk(iterable, cb) {
  iterable.forEach(function(x) {
    if (Immutable.Iterable.isIterable(x)) {
      walk(x, cb)
    } else {
      return cb(x)
    }
  })
}

module.exports.walk = walk;
