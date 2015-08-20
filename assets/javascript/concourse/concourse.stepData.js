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
