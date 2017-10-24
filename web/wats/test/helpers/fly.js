'use strict';

const { exec, spawn } = require('child-process-promise');
const uuidv4 = require('uuid/v4');
const tmp = require('tmp-promise');

class Fly {
  constructor(url) {
    this.url = url;
    this.target = `wats-target-${uuidv4()}`;

    this.teams = [];
  }

  async setup() {
    this.home = await tmp.dir({ unsafeCleanup: true });
  }

  run(command) {
    return this._run(`fly -t ${this.target} ${command}`);
  }

  spawn(command) {
    return this._spawn('fly', ['-t', this.target].concat(command.split(' ')));
  }

  async newTeam() {
    await this.loginAs('main');

    var name = `watsjs-team-${uuidv4()}`;

    await this.run(`set-team -n ${name} --no-really-i-dont-want-any-auth --non-interactive`);

    this.teams.push(name);

    return name;
  }

  async cleanup() {
    await this.loginAs('main');

    for (var i = 0; i < this.teams.length; i++) {
      await this.run(`destroy-team --non-interactive --team-name ${this.teams[i]}`);
    }

    await this.home.cleanup();
  }

  loginAs(teamName) {
    return this.run(`login -c ${this.url} -n ${teamName}`);
  }

  async cleanUpTestTeams() {
    await this.loginAs("main");

    var teams = await this.table("teams");

    var destroys = [];

    teams.forEach((team) => {
      if (team.name.indexOf("watsjs-team-") === 0) {
        let tryDeleting = this.run(`destroy-team --team-name ${team.name} --non-interactive`).catch((err) => {
          if (err.stderr.indexOf("resource not found") !== -1) {
            return;
          } else {
            throw err;
          }
        });

        destroys.push(tryDeleting);
      }
    });

    return Promise.all(destroys);
  }

  async table(command) {
    const result = await this._run(`fly -t ${this.target} --print-table-headers ${command}`);

    var rows = [];
    var headers;
    result.stdout.split('\n').forEach((line) => {
      if (line == '') {
        return;
      }

      var cols = line.trim().split(/\s{2,}/);

      if (headers) {
        var row = {};
        for (var i = 0; i < headers.length; i++) {
          row[headers[i]] = cols[i];
        }

        rows.push(row);
      } else {
        headers = cols;
      }
    });

    return rows;
  }

  _run(command) {
    return exec(command, {env: this._env()});
  }

  _spawn(path, args) {
    return spawn(path, args, {env: this._env()});
  }

  _env() {
    return {
      "HOME": this.home.path,
      "PATH": process.env["PATH"]
    }
  }
}

module.exports = Fly;
