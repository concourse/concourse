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

  text(page, ele) {
    return page.evaluate(x => (x || document.body).innerText, ele);
  }

  async loginAs(t, page, teamName) {
    await page.goto(`${this.url}/teams/${teamName}/login`);
    await page.waitFor('.login-page button');
    await page.click('.login-page button');
    await page.waitForNavigation({waitUntil: 'networkidle'});
    t.notRegex(await this.text(page), /login/);
  }

  computedStyle(page, element, style) {
    return element.executionContext().evaluate((element, style) => {
      return window.getComputedStyle(element)[style]
    }, element, style)
  }
}

module.exports = Web;
