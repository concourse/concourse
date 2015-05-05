describe("Resource", function () {
  var resource;

  beforeEach(function () {
    window.pipelineName = "some-pipeline";

    setFixtures(
      '<div class="js-resource" data-resource-name="a-resource"><span class="js-pauseResourceCheck disabled"><i class="fa-pause"></i></span><h3 class="js-resourceStatusText">checking successfully</h3></span></div>'
    );

    resource = new concourse.Resource($('.js-resource'));

    jasmine.Ajax.install();
  });

  afterEach(function() {
    window.pipelineName = undefined;
    jasmine.Ajax.uninstall();
  });

  describe('#bindEvents', function () {
    it('binds on the click of .js-pauseResourceCheck', function () {
      spyOn(resource, 'pause');

      resource.bindEvents();

      $(".js-pauseResourceCheck").trigger('click');
      expect(resource.pause).toHaveBeenCalled();
    });
  });

  describe("after the events have been bound", function () {
    beforeEach(function () {
      resource.bindEvents();
    });

    var $resource = {
      pauseBtn: function(){return $(".js-pauseResourceCheck");},
      pauseBtnIcon: function(){return $(".js-pauseResourceCheck i");},
      resourceStatus: function(){return $(".js-resourceStatusText").text();}
    };

    var respondWithSuccess = function(request) {
      var successRequest = request || jasmine.Ajax.requests.mostRecent();
      successRequest.respondWith({
        "status": 200,
        "contentType": "application/json",
        "responseText": "{}"
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
      $resource.pauseBtn().trigger('click');
    };

    describe('clicking the button', function () {
      it('swaps the enabled class with disabled and disabled with enabled', function () {
        // button start disabled
        expect($resource.pauseBtn()).toHaveClass('disabled');
        expect($resource.pauseBtnIcon()).toHaveClass('fa-pause');
        expect($resource.pauseBtn()).not.toHaveClass('loading');
        expect($resource.resourceStatus()).toBe('checking successfully');

        clickButton();

        // on click, it should now be enabled
        expect($resource.pauseBtn()).toHaveClass('loading');
        expect($resource.pauseBtnIcon()).toHaveClass('fa-circle-o-notch');
        expect($resource.pauseBtn()).not.toHaveClass('disabled');

        respondWithSuccess();

        expect($resource.pauseBtn()).not.toHaveClass('loading');
        expect($resource.pauseBtnIcon()).not.toHaveClass('fa-circle-o-notch fa-spin');
        expect($resource.pauseBtn()).toHaveClass('enabled');
        expect($resource.pauseBtnIcon()).toHaveClass('fa-play');
        expect($resource.resourceStatus()).toBe('checking paused');

        clickButton();

        // on click, it should now be disabled again
        expect($resource.pauseBtn()).toHaveClass('loading');
        expect($resource.pauseBtnIcon()).toHaveClass('fa-circle-o-notch');

        respondWithSuccess();

        expect($resource.pauseBtn()).not.toHaveClass('enabled');
        expect($resource.pauseBtnIcon()).toHaveClass('fa-pause');
        expect($resource.resourceStatus()).toBe('checking successfully');
      });

      it('sets the button as errored when the request fails', function () {
        // button start disabled
        expect($resource.pauseBtn()).toHaveClass('disabled');
        expect($resource.pauseBtn()).not.toHaveClass('loading');

        clickButton();

        // on click, it should now be enabled
        expect($resource.pauseBtn()).toHaveClass('loading');
        expect($resource.pauseBtn()).not.toHaveClass('disabled');

        respondWithError();

        expect($resource.pauseBtn()).not.toHaveClass('loading');
        expect($resource.pauseBtn()).toHaveClass('errored');
      });

      it('sets the button as errored when the unpaise fails', function () {
        clickButton();
        respondWithSuccess();

        clickButton();
        respondWithError();

        expect($resource.pauseBtn()).not.toHaveClass('loading');
        expect($resource.pauseBtn()).toHaveClass('errored');
      });

      it("makes a request", function () {
        clickButton();

        var request = jasmine.Ajax.requests.mostRecent();

        expect(request.url).toBe('/api/v1/pipelines/some-pipeline/resources/a-resource/pause');
        expect(request.method).toBe('PUT');

        respondWithSuccess(request);

        clickButton();

        request = jasmine.Ajax.requests.mostRecent();

        expect(request.url).toBe('/api/v1/pipelines/some-pipeline/resources/a-resource/unpause');
        expect(request.method).toBe('PUT');

        respondWithSuccess(request);
      });
    });
  });
});
