describe("Build", function () {
  var build;

  beforeEach(function () {

    setFixtures(
      '<div class="js-build running" data-build-id="123" data-status="running"><div class="js-abortBuild"></div></div>'
    );

    build = new concourse.Build($('.js-build'));

    jasmine.Ajax.install();
  });

  afterEach(function() {
    jasmine.Ajax.uninstall();
  });

  describe('#bindEvents', function () {
    it('binds on the click of .js-abortBuild', function () {
      spyOn(build, 'abort');

      build.bindEvents();

      $(".js-abortBuild").trigger('click');
      expect(build.abort).toHaveBeenCalled();
    });
  });

  describe('#abort', function() {
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

    var respondWithUnauthorized = function(request) {
      var errorRequest = request || jasmine.Ajax.requests.mostRecent();
      errorRequest.respondWith({
        "status": 401,
        "contentType": 'application/json'
      });
    };

    it('calls the api endpoint to abort the build', function() {
      build.abort();

      var request = jasmine.Ajax.requests.mostRecent();

      expect(request.url).toBe('/api/v1/builds/123/abort');
      expect(request.method).toBe('POST');

      respondWithSuccess(request);
    });

    describe('when the request is successful', function() {
      it('removes the js-abortBuild button', function() {
        expect($('.js-abortBuild').length).toEqual(1);

        build.abort();
        respondWithSuccess();

        expect($('.js-abortBuild').length).toEqual(0);
      });
    });

    describe('when the request is not successful', function() {
      it('sets an errored class on js-abortBuild', function() {
        expect($('.js-abortBuild')).not.toHaveClass('errored');

        build.abort();
        respondWithError();

        expect($('.js-abortBuild')).toHaveClass('errored');
      });
    });

    describe('when the request is unauthorized', function(){
      it('redirects to /login', function () {
        spyOn(concourse, 'redirect');

        expect($('.js-abortBuild').length).toEqual(1);

        build.abort();
        respondWithUnauthorized();

        expect(concourse.redirect).toHaveBeenCalledWith("/login");
      });
    });
  });
});
