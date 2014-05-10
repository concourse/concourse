// ansi_up.js
// version : 1.1.0
// author : Dru Nelson
// license : MIT
// http://github.com/drudru/ansi_up

(function (Date, undefined) {

    var ansi_up,
        VERSION = "1.1.0",

        // check for nodeJS
        hasModule = (typeof module !== 'undefined'),

        // Normal and then Bright
        ANSI_COLORS = [
          [
            { color: "0, 0, 0",        class: "ansi-black"   },
            { color: "187, 0, 0",      class: "ansi-red"     },
            { color: "0, 187, 0",      class: "ansi-green"   },
            { color: "187, 187, 0",    class: "ansi-yellow"  },
            { color: "0, 0, 187",      class: "ansi-blue"    },
            { color: "187, 0, 187",    class: "ansi-magenta" },
            { color: "0, 187, 187",    class: "ansi-cyan"    },
            { color: "255,255,255",    class: "ansi-white"   }
          ],
          [
            { color: "85, 85, 85",     class: "ansi-bright-black"   },
            { color: "255, 85, 85",    class: "ansi-bright-red"     },
            { color: "0, 255, 0",      class: "ansi-bright-green"   },
            { color: "255, 255, 85",   class: "ansi-bright-yellow"  },
            { color: "85, 85, 255",    class: "ansi-bright-blue"    },
            { color: "255, 85, 255",   class: "ansi-bright-magenta" },
            { color: "85, 255, 255",   class: "ansi-bright-cyan"    },
            { color: "255, 255, 255",  class: "ansi-bright-white"   }
          ]
        ];

    function Ansi_Up() {
      this.fg = this.bg = null;
      this.bright = 0;
    }

    Ansi_Up.prototype.escape_for_html = function (txt) {
      return txt.replace(/[&<>]/gm, function(str) {
        if (str == "&") return "&amp;";
        if (str == "<") return "&lt;";
        if (str == ">") return "&gt;";
      });
    };

    Ansi_Up.prototype.linkify = function (txt) {
      return txt.replace(/(https?:\/\/[^\s]+)/gm, function(str) {
        return "<a href=\"" + str + "\">" + str + "</a>";
      });
    };

    Ansi_Up.prototype.ansi_to_html = function (txt, options) {

      var data4 = txt.split(/\033\[/);

      var first = data4.shift(); // the first chunk is not the result of the split

      var self = this;
      var data5 = data4.map(function (chunk) {
        return self.process_chunk(chunk, options);
      });

      data5.unshift(first);

      var flattened_data = data5.reduce( function (a, b) {
        if (Array.isArray(b))
          return a.concat(b);

        a.push(b);
        return a;
      }, []);

      var escaped_data = flattened_data.join('');

      return escaped_data;
    };

    Ansi_Up.prototype.process_chunk = function (text, options) {

      // Are we using classes or styles?
      options = typeof options == 'undefined' ? {} : options;
      var use_classes = typeof options.use_classes != 'undefined' && options.use_classes;
      var key = use_classes ? 'class' : 'color';

      // Do proper handling of sequences (aka - injest vi split(';') into state machine
      //match,codes,txt = text.match(/([\d;]+)m(.*)/m);
      var matches = text.match(/([\d;]*)m([^]*)/m);

      if (!matches) return text;

      var orig_txt = matches[2];
      var nums = matches[1].split(';');

      var self = this;
      nums.map(function (num_str) {

        var num = parseInt(num_str);

        if (isNaN(num) || num === 0) {
          self.fg = self.bg = null;
          self.bright = 0;
        } else if (num === 1) {
          self.bright = 1;
        } else if ((num >= 30) && (num < 38)) {
          self.fg = ANSI_COLORS[self.bright][(num % 10)][key];
        } else if ((num >= 40) && (num < 48)) {
          self.bg = ANSI_COLORS[0][(num % 10)][key];
        }
      });

      if ((self.fg === null) && (self.bg === null)) {
        return orig_txt;
      } else {
        var styles = classes = [];
        if (self.fg) {
          if (use_classes) {
            classes.push(self.fg + "-fg");
          } else {
            styles.push("color:rgb(" + self.fg + ")");
          }
        }
        if (self.bg) {
          if (use_classes) {
            classes.push(self.bg + "-bg");
          } else {
            styles.push("background-color:rgb(" + self.bg + ")");
          }
        }
        if (use_classes) {
          return ["<span class=\"" + classes.join(' ') + "\">", orig_txt, "</span>"];
        } else {
          return ["<span style=\"" + styles.join(';') + "\">", orig_txt, "</span>"];
        }
      }
    };

    // Module exports
    ansi_up = {

      escape_for_html: function (txt) {
        var a2h = new Ansi_Up();
        return a2h.escape_for_html(txt);
      },

      linkify: function (txt) {
        var a2h = new Ansi_Up();
        return a2h.linkify(txt);
      },

      ansi_to_html: function (txt, options) {
        var a2h = new Ansi_Up();
        return a2h.ansi_to_html(txt, options);
      },

      ansi_to_html_obj: function () {
        return new Ansi_Up();
      }
    };

    // CommonJS module is defined
    if (hasModule) {
        module.exports = ansi_up;
    }
    /*global ender:false */
    if (typeof window !== 'undefined' && typeof ender === 'undefined') {
        window.ansi_up = ansi_up;
    }
    /*global define:false */
    if (typeof define === "function" && define.amd) {
        define("ansi_up", [], function () {
            return ansi_up;
        });
    }
})(Date);
