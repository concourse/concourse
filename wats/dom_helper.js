'use strict';

class Dom extends Helper {
  grabComputedStyle(locator, attr) {
    let browser = this.helpers['Nightmare'].browser;

    return browser.findElement({ css: locator }).then((el) => {
      return browser.evaluate(function (el, attr) {
        return window.getComputedStyle(window.codeceptjs.fetchElement(el))[attr];
      }, el, attr);
    });
  }
}

module.exports = Dom;
