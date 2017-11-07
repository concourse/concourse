const puppeteer = require('puppeteer');

const Fly = require('./fly');
const Web = require('./web');

// silence warning caused by starting many puppeteer
process.setMaxListeners(Infinity);

class Suite {
  constructor() {
    this.url = process.env.ATC_URL || 'http://127.0.0.1:8080';

    this.fly = new Fly(this.url);
    this.web = new Web(this.url);
  }

  async start(t) {
    await this.fly.setup();

    this.browser = await puppeteer.launch({
      args: ['--no-sandbox', '--disable-setuid-sandbox']
    });

    this.page = await this.browser.newPage();
    this.page.on("console", (msg) => {
      console.log(`BROWSER (${msg.type}):`, msg.text);
    });

    this.teamName = await this.fly.newTeam();

    t.log("team:", this.teamName);

    await this.fly.loginAs(this.teamName);
    await this.web.loginAs(t, this.page, this.teamName);

    this.succeeded = false;
  }

  passed(t) {
    this.succeeded = true;
  }

  async finish(t) {
    await this.fly.cleanup();

    if (this.page && !this.succeeded) {
      await this.page.screenshot({path: 'failure.png'});
    }

    if (this.browser) {
      await this.browser.close();
    }
  }
}

module.exports = Suite;
