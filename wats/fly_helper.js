'use strict';

const { exec } = require('child-process-promise');
const uuidv4 = require('uuid/v4');

let assert = require('assert');

class Fly extends Helper {
  fly(command) {
    return this._run(`fly -t wats ${command}`);
  }

  grabANewTeam() {
    var name = `watsjs-team-${uuidv4()}`;

    return this.flyWithInput("y", `set-team -n ${name} --no-really-i-dont-want-any-auth`).then(() => {
      return name;
    });
  }

  flyLoginAs(teamName) {
    return this.fly(`login -c ${this.helpers['Nightmare'].config.url} -n ${teamName}`);
  }

  cleanUpTestTeams() {
    return this.flyLoginAs("main").then(() => {
      return this.flyTable("teams").then((teams) => {
        var destroys = [];

        teams.forEach((team) => {
          if (team.name.indexOf("watsjs-team-") === 0) {
            destroys.push(this.flyWithInput(team.name, `destroy-team -n ${team.name}`));
          }
        });

        return Promise.all(destroys);
      });
    });
  }

  flyTable(command) {
    return this._run(`fly -t wats --print-table-headers ${command}`).then((result) => {
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
    });
  }

  flyWithInput(input, command) {
    return this._run(`echo '${input}' | fly -t wats ${command}`);
  }

  flyExpectingFailure(command) {
    return this._run(`fly -t wats ${command}`, true);
  }

  _run(command, expectFailure) {
    return exec(command).catch((error) => {
      if (expectFailure) {
        assert.ok(error);
      } else {
        assert.ifError(error);
      }
    });
  }
}

module.exports = Fly;
