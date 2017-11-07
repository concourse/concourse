'use strict';

const { exec } = require('child-process-promise');
const uuidv4 = require('uuid/v4');

class Web {
  constructor(url) {
    this.url = url;
  }

  route(path) {
    return `${this.url}${path}`;
  }

  betaRoute(path) {
    return `${this.url}/beta${path}`;
  }

  text(page, ele) {
    return page.evaluate(x => (x || document.body).innerText, ele);
  }

  async loginAs(t, page, teamName) {
    await page.goto(`${this.url}/teams/${teamName}/login`);
    await this.clickAndWait(page, '.login-page button');
    t.notRegex(await this.text(page), /login/);
  }

  async clickAndWait(page, selector) {
    await page.waitFor(selector);
    await page.click(selector);
    await page.waitForNavigation({
      waitUntil: 'networkidle',
      networkIdleInflight: 0
    });
  }

  computedStyle(page, element, style) {
    return element.executionContext().evaluate((element, style) => {
      return window.getComputedStyle(element)[style]
    }, element, style)
  }
}

module.exports = Web;
