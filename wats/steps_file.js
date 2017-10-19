'use strict';

module.exports = function() {
  return actor({
    loginAs: function(teamName) {
      this.amOnPage(`/teams/${teamName}/login`);
      this.waitForText('login', 1, '.login-page');
      this.click('login', '.login-page');
      this.dontSee('login');
    }
  });
}
