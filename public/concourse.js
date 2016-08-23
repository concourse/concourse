var concourse = {
  redirect: function(href) {
    window.location = href;
  }
};

$(function () {
  $(".js-expandable").on("click", function() {
    if($(this).parent().hasClass("expanded")) {
      $(this).parent().removeClass("expanded");
    } else {
      $(this).parent().addClass("expanded");
    }
  });
});

concourse.Build = function ($el) {
  this.$el = $el;
  this.$abortBtn = this.$el.find('.js-abortBuild');
  this.buildID = this.$el.data('build-id');
  this.abortEndpoint = '/api/v1/builds/' + this.buildID + '/abort';
};

concourse.Build.prototype.bindEvents = function () {
  var _this = this;
  this.$abortBtn.on('click', function(event) {
    _this.abort();
  });
};

concourse.Build.prototype.abort = function() {
  var _this = this;

  $.ajax({
    method: 'POST',
    url: _this.abortEndpoint
  }).done(function (resp, jqxhr) {
    _this.$abortBtn.remove();
  }).error(function (resp) {
    _this.$abortBtn.addClass('errored');

    if (resp.status == 401) {
      concourse.redirect("/login");
    }
  });
};

$(function () {
  if ($('.js-build').length) {
    var build = new concourse.Build($('.js-build'));
    build.bindEvents();
  }
});

// <button class="btn-pause disabled js-pauseResourceCheck"><i class="fa fa-fw fa-pause"></i></button>

(function ($) {
    $.fn.pausePlayBtn = function () {
      var $el = $(this);
      return {
        loading: function() {
          $el.removeClass('disabled enabled').addClass('loading');
          $el.find('i').removeClass('fa-pause').addClass('fa-circle-o-notch fa-spin');
        },

        enable: function() {
          $el.removeClass('loading').addClass('enabled');
          $el.find('i').removeClass('fa-circle-o-notch fa-spin').addClass('fa-play');
        },

        error: function() {
          $el.removeClass('loading').addClass('errored');
          $el.find('i').removeClass('fa-circle-o-notch fa-spin').addClass('fa-pause');
        },

        disable: function() {
          $el.removeClass('loading').addClass('disabled');
          $el.find('i').removeClass('fa-circle-o-notch fa-spin').addClass('fa-pause');
        }
      };
    };
})(jQuery);

$(function () {
  if ($('.js-job').length) {
    var pauseUnpause = new concourse.PauseUnpause($('.js-job'));
    pauseUnpause.bindEvents();

		$('.js-build').each(function(i, el){
			var startTime, endTime,
				$build = $(el),
				status = $build.data('status'),
				$buildTimes = $build.find(".js-build-times"),
				start = $buildTimes.data('start-time'),
				end = $buildTimes.data('end-time'),
				$startTime = $("<time>"),
				$endTime = $("<time>");

			if(window.moment === undefined){
				console.log("moment library not included, cannot parse durations");
				return;
			}

			if (start > 0) {
				startTime = moment.unix(start);
				$startTime.text(startTime.fromNow());
				$startTime.attr("datetime", startTime.format());
				$startTime.attr("title", startTime.format("lll Z"));
				$("<div/>").text("started: ").append($startTime).appendTo($buildTimes);
			}

			endTime = moment.unix(end);
			$endTime.text(endTime.fromNow());
			$endTime.attr("datetime", endTime.format());
			$endTime.attr("title", endTime.format("lll Z"));
			$("<div/>").text(status + ": ").append($endTime).appendTo($buildTimes);

			if (end > 0 && start > 0) {
				var duration = moment.duration(endTime.diff(startTime));

				var durationEle = $("<span>");
				durationEle.addClass("duration");
				durationEle.text(duration.format("h[h]m[m]s[s]"));

				$("<div/>").text("duration: ").append(durationEle).appendTo($buildTimes);
			}
		});
	}
});

concourse.PauseUnpause = function ($el, pauseCallback, unpauseCallback) {
  this.$el = $el;
  this.pauseCallback = pauseCallback === undefined ? function(){} : pauseCallback;
  this.unpauseCallback = unpauseCallback === undefined ? function(){} : unpauseCallback;
  this.pauseBtn = this.$el.find('.js-pauseUnpause').pausePlayBtn();
  this.pauseEndpoint = this.$el.data('endpoint') + "/pause";
  this.unPauseEndpoint = this.$el.data('endpoint') + "/unpause";
  this.teamName = this.$el.data('teamname');
};

concourse.PauseUnpause.prototype.bindEvents = function () {
  var _this = this;

  _this.$el.delegate('.js-pauseUnpause.disabled', 'click', function (event) {
    _this.pause();
  });

  _this.$el.delegate('.js-pauseUnpause.enabled', 'click', function (event) {
    _this.unpause();
  });
};

concourse.PauseUnpause.prototype.pause = function (pause) {
  var _this = this;
  _this.pauseBtn.loading();

  $.ajax({
    method: 'PUT',
    url: _this.pauseEndpoint,
  }).done(function (resp, jqxhr) {
    _this.pauseBtn.enable();
    _this.pauseCallback();
  }).error(function (resp) {
    _this.requestError(resp);
  });
};


concourse.PauseUnpause.prototype.unpause = function (event) {
  var _this = this;
  _this.pauseBtn.loading();

  $.ajax({
    method: 'PUT',
    url: _this.unPauseEndpoint
  }).done(function (resp) {
    _this.pauseBtn.disable();
    _this.unpauseCallback();
  }).error(function (resp) {
    _this.requestError(resp);
  });
};

concourse.PauseUnpause.prototype.requestError = function (resp) {
  this.pauseBtn.error();

  if (resp.status == 401) {
    concourse.redirect("/teams/" + this.teamName + "/login");
  }
};

$(function () {
  if ($('.js-resource').length) {
    var pauseUnpause = new concourse.PauseUnpause(
      $('.js-resource'),
      function() {}, // on pause
      function() {}  // on unpause
    );
    pauseUnpause.bindEvents();
  }
});

(function(){
  concourse.StepData = function(data){
    if(data === undefined){
      this.data = {};
    } else {
      this.data = data;
    }
    this.idCounter = 1;
    this.parallelGroupStore = {};
    return this;
  };

  var stepDataProto = {
    updateIn: function(location, upsertFunction){
      var newData = jQuery.extend(true, {}, this.data);
      var keyPath;

      if(Array.isArray(location)){
        keyPath = location.join('.');
      } else {
        keyPath = location.id;
      }

      var before = newData[keyPath];
      newData[keyPath] = upsertFunction(newData[keyPath]);
      var after = newData[keyPath];

      if (before === after) {
        return this;
      }

      return new concourse.StepData(newData);
    },

    getIn: function(location) {
      if(Array.isArray(location)){
        return this.data[location.join('.')];
      } else {
        return this.data[location.id];
      }
    },

    setIn: function(location, val) {
      var newData = jQuery.extend(true, {}, this.data);

      if (Array.isArray(location)) {
        newData[location.join('.')] = val;
      }
      else {
        newData[location.id] = val;
      }
      return new concourse.StepData(newData);

    },

    forEach: function(cb) {
      for(var key in this.data) {
        cb(this.data[key]);
      }
    },

    getSorted: function() {
      var ret = [];
      for(var key in this.data) {
        ret.push([key, this.data[key]]);
      }

      ret = ret.sort(function(a, b){
        var aLoc = a[0].split('.'),
            bLoc = b[0].split('.');

        for(var i = 0; i < aLoc.length; i++){
          var aVal = parseInt(aLoc[i]);
          var bVal = parseInt(bLoc[i]);

          if(aVal > bVal){
            return 1;
          }
        }

        return -1;
      });

      ret = ret.map(function(val){
        return val[1];
      });

      return ret;
    },

    translateLocation: function(location, substep) {
      if (!Array.isArray(location)) {
        return location;
      }

      var id,
          parallel_group = 0,
          parent_id = 0;

      if(location.length > 1) {
        var parallelGroupLocation = location.slice(0, location.length - 1).join('.');

        if(this.parallelGroupStore[parallelGroupLocation] === undefined){
          this.parallelGroupStore[parallelGroupLocation] = this.idCounter;
          this.idCounter++;
        }

        parallel_group = this.parallelGroupStore[parallelGroupLocation];

        if(location.length > 2) {
          var parentGroupLocation = location.slice(0, location.length - 2).join('.');

          if(this.parallelGroupStore[parentGroupLocation] === undefined){
            parent_id = 0;
          } else {
            parent_id = this.parallelGroupStore[parentGroupLocation];
          }
        }
      }


      id = this.idCounter;
      this.idCounter++;

      if(substep){
        parent_id = id - 1;
        parallel_group = 0;
      }


      return {
        id: id,
        parallel_group: parallel_group,
        parent_id: parent_id
      };
    },

    getRenderableData: function() {
      var _this = this,
          ret = [],
          allObjects = [],
          sortedData = _this.getSorted();

      var addStepToGroup = function(primaryGroupID, secondaryGroupID, parentID, renderGroup){
        if(allObjects[primaryGroupID] === undefined){
          allObjects[primaryGroupID] = renderGroup;
        } else if (allObjects[primaryGroupID].hold) {
          renderGroup.groupSteps = allObjects[primaryGroupID].groupSteps;
          renderGroup.children = allObjects[primaryGroupID].children;
          allObjects[primaryGroupID] = renderGroup;
        }


        if(secondaryGroupID < primaryGroupID) {
          allObjects[primaryGroupID].groupSteps[location.id] = allObjects[location.id];
        }

        if (secondaryGroupID !== 0 && secondaryGroupID < primaryGroupID) {
          allObjects[secondaryGroupID].groupSteps[primaryGroupID] = allObjects[primaryGroupID];
        }

        if (parentID !== 0) {
          if(step.isHook()){
            allObjects[parentID].children[primaryGroupID] = allObjects[primaryGroupID];
          } else {
            allObjects[parentID].groupSteps[primaryGroupID] = allObjects[primaryGroupID];
          }
        }
      };

      for(var i = 0; i < sortedData.length; i++){
        var step = sortedData[i];
        var location = _this.translateLocation(step.origin().location, step.origin().substep);
        var stepLogs = step.logs();
        var logLines = stepLogs.lines;

        var render = {
          key: location.id,
          step: step,
          location: location,
          logLines: logLines,
          children: []
        };

        allObjects[location.id] = render;

        if (location.parent_id !== 0 && allObjects[location.parent_id] === undefined) {
          allObjects[location.parent_id] = {hold: true, groupSteps: [], children: []};
        }

        if (location.parallel_group !== 0 && allObjects[location.parallel_group] === undefined) {
          allObjects[location.parallel_group] = {hold: true, groupSteps: [], children: []};
        }

        location.serial_group = location.serial_group ? location.serial_group : 0;

        if (location.serial_group !== 0) {
          renderSerialGroup = {
            serial: true,
            step: step,
            location: location,
            key: location.serial_group,
            groupSteps: [],
            children: []
          };

          addStepToGroup(location.serial_group, location.parallel_group, location.parent_id, renderSerialGroup);
        }

        if(location.parallel_group !== 0) {
          renderParallelGroup = {
            aggregate: true,
            step: step,
            location: location,
            key: location.parallel_group,
            groupSteps: [],
            children: []
          };

          addStepToGroup(location.parallel_group, location.serial_group, location.parent_id, renderParallelGroup);
        }


        if (location.parallel_group !== 0 &&
          (location.serial_group === 0 || location.serial_group > location.parallel_group)
        ) {
          ret[location.parallel_group] = allObjects[location.parallel_group];
        } else if (location.serial_group !== 0) {
          ret[location.serial_group] = allObjects[location.serial_group];
        } else {
          ret[location.id] = allObjects[location.id];

          if(location.parent_id !== 0){
            allObjects[location.parent_id].children[location.id] = allObjects[location.id];
          }
        }
      }

      return ret;
    }

  };

  concourse.StepData.prototype = stepDataProto;
})();
