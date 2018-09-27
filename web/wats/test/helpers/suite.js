const Fly = require('./fly');
const Web = require('./web');
const uuidv4 = require('uuid/v4');

// silence warning caused by starting many puppeteer
process.setMaxListeners(Infinity);

class Suite {
  constructor() {
    this.url = process.env.ATC_URL || 'http://localhost:8080';
    this.admin_username = process.env.ATC_ADMIN_USERNAME || 'test';
    this.admin_password = process.env.ATC_ADMIN_PASSWORD || 'test';
    this.guest_username = process.env.ATC_GUEST_USERNAME || 'guest';
    this.guest_password = process.env.ATC_GUEST_PASSWORD || 'guest';

    this.teamName = `watsjs-team-${uuidv4()}`;
    this.teams = [];

    this.fly = new Fly(this.url, this.admin_username, this.admin_password, this.teamName);
    this.web = new Web(this.url, this.admin_username, this.admin_password);
  }

  static async build(t) {
    let suite = new Suite();
    await suite.init(t);
    return suite;
  }

  async init(t) {
    await this.newTeam(this.admin_username, this.teamName);
    await this.fly.init();
    await this.web.init();
    await this.web.login(t);

    this.succeeded = false;
  }

  async newTeam(username = this.admin_username, teamName) {
    if (!teamName) {
      teamName = `watsjs-team-${uuidv4()}`;
    }
    let fly = await Fly.build(this.url, this.admin_username, this.admin_password, 'main');

    await fly.newTeam(teamName, username);
    this.teams.push(teamName);

    return teamName;
  }

  async destroyTeams() {
    let fly = await Fly.build(this.url, this.admin_username, this.admin_password, 'main');

    var team;
    while (team = this.teams.pop()) {
      await fly.destroyTeam(team);
    }
  }

  passed(t) {
    this.succeeded = true;
  }

  async finish(t) {
    await this.destroyTeams();
    await this.fly.cleanup();

    if (this.web.page && !this.succeeded) {
      await this.web.page.screenshot({path: 'failure.png'});
    }

    if (this.web.browser) {
      await this.web.browser.close();
    }
  }
}

module.exports = Suite;
