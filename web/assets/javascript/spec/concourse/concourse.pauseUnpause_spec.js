describe("Resource", function () {
  var pauseUnpause;
  var pauseCallbackSpy;
  var unpauseCallbackSpy;

  beforeEach(function () {
    setFixtures(
      '<div class="js-something" data-teamname="some-team" data-endpoint="/something/other"><span class="js-pauseUnpause disabled"><i class="fa-pause"></i></span></div>'
    );

    pauseCallbackSpy = jasmine.createSpy('pauseCallback');
    unpauseCallbackSpy = jasmine.createSpy('unpauseCallback');
    pauseUnpause = new concourse.PauseUnpause($('.js-something'), pauseCallbackSpy, unpauseCallbackSpy);

    jasmine.Ajax.install();
  });

  afterEach(function() {
    jasmine.Ajax.uninstall();
  });

  var $pauseUnpause = {
    pauseBtn: function(){return $(".js-pauseUnpause");},
    pauseBtnIcon: function(){return $(".js-pauseUnpause i");}
  };

  var respondWithSuccess = function(request) {
    var successRequest = request || jasmine.Ajax.requests.mostRecent();
    successRequest.respondWith({
      "status": 200,
      "contentType": "application/json",
      "responseText": "{}"
    });
  };

  var respondWithUnauthorized = function(request) {
    var errorRequest = request || jasmine.Ajax.requests.mostRecent();
    errorRequest.respondWith({
      "status": 401,
      "contentType": 'application/json'
    });
  };

  var respondWithError = function(request) {
    var errorRequest = request || jasmine.Ajax.requests.mostRecent();
    errorRequest.respondWith({
      "status": 500,
      "contentType": 'application/json'
    });
  };

  var clickButton = function() {
    $pauseUnpause.pauseBtn().trigger('click');
  };

  describe('#bindEvents', function () {
    it('binds on the click of .js-pauseUnpause', function () {
      spyOn(pauseUnpause, 'pause');

      pauseUnpause.bindEvents();

      $(".js-pauseUnpause").trigger('click');
      expect(pauseUnpause.pause).toHaveBeenCalled();
    });
  });

  describe("after the events have been bound", function () {
    beforeEach(function () {
      pauseUnpause.bindEvents();
      spyOn(concourse, 'redirect');
    });

    describe('clicking the button', function () {
      it('swaps the enabled class with disabled and disabled with enabled', function () {
        // button start disabled
        expect($pauseUnpause.pauseBtn()).toHaveClass('disabled');
        expect($pauseUnpause.pauseBtnIcon()).toHaveClass('fa-pause');
        expect($pauseUnpause.pauseBtn()).not.toHaveClass('loading');

        clickButton();

        // on click, it should now be enabled
        expect($pauseUnpause.pauseBtn()).toHaveClass('loading');
        expect($pauseUnpause.pauseBtnIcon()).toHaveClass('fa-circle-o-notch');
        expect($pauseUnpause.pauseBtn()).not.toHaveClass('disabled');

        respondWithSuccess();

        expect($pauseUnpause.pauseBtn()).not.toHaveClass('loading');
        expect($pauseUnpause.pauseBtnIcon()).not.toHaveClass('fa-circle-o-notch fa-spin');
        expect($pauseUnpause.pauseBtn()).toHaveClass('enabled');
        expect($pauseUnpause.pauseBtnIcon()).toHaveClass('fa-play');

        clickButton();

        // on click, it should now be disabled again
        expect($pauseUnpause.pauseBtn()).toHaveClass('loading');
        expect($pauseUnpause.pauseBtnIcon()).toHaveClass('fa-circle-o-notch');

        respondWithSuccess();

        expect($pauseUnpause.pauseBtn()).not.toHaveClass('enabled');
        expect($pauseUnpause.pauseBtnIcon()).toHaveClass('fa-pause');
      });

      it('redirects to /login when the request is unauthorized', function () {
        // button start disabled
        expect($pauseUnpause.pauseBtn()).toHaveClass('disabled');
        expect($pauseUnpause.pauseBtn()).not.toHaveClass('loading');

        clickButton();

        // on click, it should now be enabled
        expect($pauseUnpause.pauseBtn()).toHaveClass('loading');
        expect($pauseUnpause.pauseBtn()).not.toHaveClass('disabled');

        respondWithUnauthorized();

        expect($pauseUnpause.pauseBtn()).not.toHaveClass('loading');
        expect($pauseUnpause.pauseBtn()).toHaveClass('errored');

        expect(concourse.redirect).toHaveBeenCalledWith("/teams/some-team/login");
      });

      it('sets the button as errored when the request fails', function () {
        // button start disabled
        expect($pauseUnpause.pauseBtn()).toHaveClass('disabled');
        expect($pauseUnpause.pauseBtn()).not.toHaveClass('loading');

        clickButton();

        // on click, it should now be enabled
        expect($pauseUnpause.pauseBtn()).toHaveClass('loading');
        expect($pauseUnpause.pauseBtn()).not.toHaveClass('disabled');

        respondWithError();

        expect($pauseUnpause.pauseBtn()).not.toHaveClass('loading');
        expect($pauseUnpause.pauseBtn()).toHaveClass('errored');

        expect(concourse.redirect).not.toHaveBeenCalled();
      });

      it('sets the button as errored when the unpause fails', function () {
        clickButton();
        respondWithSuccess();

        clickButton();
        respondWithError();

        expect($pauseUnpause.pauseBtn()).not.toHaveClass('loading');
        expect($pauseUnpause.pauseBtn()).toHaveClass('errored');

        expect(concourse.redirect).not.toHaveBeenCalled();
      });

      it('sets the button as errored when the unpause is unauthorized', function () {
        clickButton();
        respondWithSuccess();

        clickButton();
        respondWithUnauthorized();

        expect($pauseUnpause.pauseBtn()).not.toHaveClass('loading');
        expect($pauseUnpause.pauseBtn()).toHaveClass('errored');

        expect(concourse.redirect).toHaveBeenCalledWith("/teams/some-team/login");
      });

      it("makes a request", function () {
        clickButton();

        var request = jasmine.Ajax.requests.mostRecent();

        expect(request.url).toBe('/something/other/pause');
        expect(request.method).toBe('PUT');

        respondWithSuccess(request);

        clickButton();

        request = jasmine.Ajax.requests.mostRecent();

        expect(request.url).toBe('/something/other/unpause');
        expect(request.method).toBe('PUT');

        respondWithSuccess(request);
      });

      it("calls the provided callbacks", function() {
        clickButton();
        respondWithSuccess();
        expect(pauseCallbackSpy).toHaveBeenCalled();
        expect(unpauseCallbackSpy).not.toHaveBeenCalled();

        clickButton();
        respondWithSuccess();
        expect(unpauseCallbackSpy).toHaveBeenCalled();
      });
    });
  });

  describe("when i do not pass in callbacks", function() {
    beforeEach(function(){
      pauseUnpause = new concourse.PauseUnpause($('.js-something'));
      pauseUnpause.bindEvents();
    });

    it("does not error when making a request", function () {
      expect(function() {
        clickButton();
        respondWithSuccess();
      }).not.toThrow();
    });
  });
});
