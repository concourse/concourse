describe("Job", function () {
  var job;

  beforeEach(function () {

    setFixtures(
      '<div class="js-job" data-job-name="a-job"><div class="succeeded"><span class="js-pauseJobCheck disabled"><i class="fa-pause"></i></span></div></span></div>'
    );

    job = new concourse.Job($('.js-job'));

    jasmine.Ajax.install();
  });

  afterEach(function() {
    jasmine.Ajax.uninstall();
  });

  describe('#bindEvents', function () {
    it('binds on the click of .js-pauseJobCheck', function () {
      spyOn(job, 'pause');

      job.bindEvents();

      $(".js-pauseJobCheck").trigger('click');
      expect(job.pause).toHaveBeenCalled();
    });
  });

  describe("after the events have been bound", function () {
    beforeEach(function () {
      job.bindEvents();
    });

    var $job = {
      pauseBtn: function(){return $(".js-pauseJobCheck");},
      pauseBtnIcon: function(){return $(".js-pauseJobCheck i");},
      currentBuild: function(){return $(".js-currentBuild");}
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
      $job.pauseBtn().trigger('click');
    };

    describe('clicking the button', function () {
      it('swaps the enabled class with disabled and disabled with enabled', function () {
        // button start disabled
        expect($job.pauseBtn()).toHaveClass('disabled');
        expect($job.pauseBtnIcon()).toHaveClass('fa-pause');
        expect($job.pauseBtn()).not.toHaveClass('loading');

        clickButton();

        // on click, it should now be enabled
        expect($job.pauseBtn()).toHaveClass('loading');
        expect($job.pauseBtnIcon()).toHaveClass('fa-circle-o-notch');
        expect($job.pauseBtn()).not.toHaveClass('disabled');

        respondWithSuccess();

        expect($job.pauseBtn()).not.toHaveClass('loading');
        expect($job.pauseBtnIcon()).not.toHaveClass('fa-circle-o-notch fa-spin');
        expect($job.pauseBtn()).toHaveClass('enabled');
        expect($job.pauseBtnIcon()).toHaveClass('fa-play');

        clickButton();

        // on click, it should now be disabled again
        expect($job.pauseBtn()).toHaveClass('loading');
        expect($job.pauseBtnIcon()).toHaveClass('fa-circle-o-notch');

        respondWithSuccess();

        expect($job.pauseBtn()).not.toHaveClass('enabled');
        expect($job.pauseBtnIcon()).toHaveClass('fa-pause');


      });

      it('sets the button as errored when the request fails', function () {
        // button start disabled
        expect($job.pauseBtn()).toHaveClass('disabled');
        expect($job.pauseBtn()).not.toHaveClass('loading');

        clickButton();

        // on click, it should now be enabled
        expect($job.pauseBtn()).toHaveClass('loading');
        expect($job.pauseBtn()).not.toHaveClass('disabled');

        respondWithError();

        expect($job.pauseBtn()).not.toHaveClass('loading');
        expect($job.pauseBtn()).toHaveClass('errored');
      });

      it('sets the button as errored when the unpaise fails', function () {
        clickButton();
        respondWithSuccess();

        clickButton();
        respondWithError();

        expect($job.pauseBtn()).not.toHaveClass('loading');
        expect($job.pauseBtn()).toHaveClass('errored');
      });

      it("makes a request", function () {
        clickButton();

        var request = jasmine.Ajax.requests.mostRecent();

        expect(request.url).toBe('/api/v1/jobs/a-job/pause');
        expect(request.method).toBe('PUT');

        respondWithSuccess(request);

        clickButton();

        request = jasmine.Ajax.requests.mostRecent();

        expect(request.url).toBe('/api/v1/jobs/a-job/unpause');
        expect(request.method).toBe('PUT');

        respondWithSuccess(request);
      });
    });
  });
});
