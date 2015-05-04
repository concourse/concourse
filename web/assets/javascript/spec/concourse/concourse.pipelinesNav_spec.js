describe("Pipelines Nav", function () {
  var pipelinesNav;

  beforeEach(function () {

    setFixtures(
      '<body><div class="js-pipelinesNav"><ul class="js-pipelinesNav-list"></ul><span class="js-pipelinesNav-toggle"></span></div></body>'
    );

    pipelinesNav = new concourse.PipelinesNav($('.js-pipelinesNav'));

    jasmine.Ajax.install();
  });

  afterEach(function() {
    jasmine.Ajax.uninstall();
  });

  describe('#bindEvents', function () {
    it('binds on the click of .js-pipelinesNav-toggle', function () {
      pipelinesNav.bindEvents();

      $(".js-pipelinesNav-toggle").trigger('click');
      expect($('body')).toHaveClass('pipelinesNav-visible');

      $(".js-pipelinesNav-toggle").trigger('click');
      expect($('body')).not.toHaveClass('pipelinesNav-visible');
    });

    it('calls to load the pipelines', function() {
      spyOn(pipelinesNav, 'loadPipelines');

      pipelinesNav.bindEvents();

      expect(pipelinesNav.loadPipelines).toHaveBeenCalled();
    });
  });

  describe('#loadPipelines', function() {
    var respondWithSuccess = function(request) {
      var successRequest = request || jasmine.Ajax.requests.mostRecent();
      var successJson = [
      {
        "id": 1,
        "name": "a-pipeline",
        "url": "/pipelines/a-pipeline"
      },{
        "id": 2,
        "name": "another-pipeline",
        "url": "/pipelines/another-pipeline"
      }];

      successRequest.respondWith({
        "status": 200,
        "contentType": "application/json",
        "responseText":JSON.stringify(successJson)
      });
    };

    var respondWithError = function(request) {
      var errorRequest = request || jasmine.Ajax.requests.mostRecent();
      errorRequest.respondWith({
        "status": 500,
        "contentType": 'application/json'
      });
    };

    it('calls the api endpoint to get the pipelinesNav', function() {
      pipelinesNav.loadPipelines();

      var request = jasmine.Ajax.requests.mostRecent();

      expect(request.url).toBe('/api/v1/pipelines');
      expect(request.method).toBe('GET');

      respondWithSuccess(request);
    });

    describe('when the request is successful', function() {
      it('loads the results into the list', function() {
        expect($('.js-pipelinesNav-list li').length).toEqual(0);

        pipelinesNav.loadPipelines();

        respondWithSuccess();

        expect($('.js-pipelinesNav-list li').length).toEqual(2);

        expect($('.js-pipelinesNav-list li:nth-of-type(1)').html()).toEqual(
          '<a href="/pipelines/a-pipeline">a-pipeline</a>'
        );

        expect($('.js-pipelinesNav-list li:nth-of-type(2)').html()).toEqual(
          '<a href="/pipelines/another-pipeline">another-pipeline</a>'
        );
      });
    });
  });
});
