describe("Step Data", function () {
  var stepData;

  beforeEach(function () {
    stepData = new concourse.StepData();
  });

  describe("#updateIn, #getIn", function(){
    it("can store and retrieve events by objects with ids", function() {
      helloStepData = stepData.updateIn({id: 1}, function(){
        return "hello world";
      });

      expect(helloStepData.getIn({id: 1})).toEqual("hello world");
      expect(stepData.getIn({id: 1})).toEqual(undefined);
    });

    it("does not create a new object if no changes are made", function() {
      before = stepData.updateIn({id: 1}, function(){
        return "hello world";
      });

      after = before.updateIn({id: 1}, function(){
        return "hello world";
      });

      expect(before).toBe(after);
    });

    it("passes the value if found to the argument of the callback function", function() {
      helloStepData = stepData.updateIn({id: 1}, function(data){
        expect(data).toEqual(undefined);
        return "hello world";
      });

      helloStepData.updateIn({id:1}, function(data){
        expect(data).toEqual("hello world");
      });
    });

    it("can store and retrieve events by arrays of ints", function() {
      stepData = stepData.updateIn([0,0], function(){
        return "hello world";
      });

      stepData = stepData.updateIn([0,1], function(){
        return "goodbye world";
      });

      expect(stepData.getIn([0,0])).toEqual("hello world");
      expect(stepData.getIn([0,1])).toEqual("goodbye world");
    });
  });

  describe("#setIn", function(){
    it("sets data for a given location", function(){
      idStepData = stepData.setIn({id: 3}, "3");

      expect(idStepData.getIn({id: 3})).toEqual("3");

      idStepData = idStepData.setIn({id: 3}, "4");
      expect(idStepData.getIn({id: 3})).toEqual("4");


      aryStepData = stepData.setIn([1, 0, 0], "[1, 0, 0]");
      expect(aryStepData.getIn([1, 0, 0])).toEqual("[1, 0, 0]");

       aryStepData= aryStepData.setIn([1, 0, 0], "[1, 0, 1]");
       expect(aryStepData.getIn([1, 0, 0])).toEqual("[1, 0, 1]");
    });
  });

  describe("#getSorted", function(){
    it("can retrieve all objects that have been stored", function(){
      stepData = stepData.updateIn([0,2,1], function(){
        return "0,2,1";
      });

      stepData = stepData.updateIn([0,1], function(){
        return "0,1";
      });

      stepData = stepData.updateIn([0,2,0], function(){
        return "0,2,0";
      });

      stepData = stepData.updateIn([0,0], function(){
        return "0,0";
      });

      expect(stepData.getSorted()).toEqual(["0,0", "0,1", "0,2,0", "0,2,1"]);
    });

    it("handles location data in the form of ids", function(){
      stepData = stepData.updateIn({id: 3}, function(){
        return "herp";
      });

      stepData = stepData.updateIn({id: 1}, function(){
        return "banana";
      });

      stepData = stepData.updateIn({id: 4}, function(){
        return "derp";
      });

      stepData = stepData.updateIn({id: 2}, function(){
        return "apple";
      });

      expect(stepData.getSorted()).toEqual(["banana", "apple", "herp", "derp"]);
    });
  });

  describe("#forEach", function(){
    it("can itterate over it's data with a callback", function() {
      stepData = stepData.updateIn({id:1}, function(){return "1";}).
          updateIn({id:4}, function(){return "4";}).
          updateIn({id:3}, function(){return "3";}).
          updateIn({id:2}, function(){return "2";});

      var ret = [];
      stepData.forEach(function(val){
        ret.push(val);
      });

      expect(ret.length).toEqual(4);
      expect(ret).toContain("1");
      expect(ret).toContain("2");
      expect(ret).toContain("3");
      expect(ret).toContain("4");
    });
  });

  describe("#getRenderableData", function(){
    function getFakeStep(id, parent_id, parallel_group, serial_group, hook){
      return {
        origin: function(){
          return {
            name: "origin",
            type: "get",
            source: "source",
            location: {
              parent_id: parent_id,
              id: id,
              parallel_group: parallel_group,
              serial_group: serial_group
            },
            hook: hook
          };
        },
        isHook: function(){
          return hook !== "";
        },
        logs: function() {
          return {
            lines: true
          };
        }
      };
    }

    it("can build render data for parent-child steps", function(){
      var renderableData;
      var firstStep = getFakeStep(1, 0, 0, 0, "");
      var secondStep = getFakeStep(2, 1, 0, 0, "");

      stepData = stepData.
        updateIn({id:1}, function(){return firstStep;}).
        updateIn({id:2}, function(){return secondStep;});

      renderableData = stepData.getRenderableData();
      expect(renderableData[1].key).toEqual(1);
      expect(renderableData[1].step).toEqual(firstStep);
      expect(renderableData[1].children[2]).toEqual({
        key: 2,
        step: secondStep,
        location: {id: 2, parent_id: 1, parallel_group: 0, serial_group: 0},
        logLines: true,
        children: [],
      });

      expect(renderableData[2]).toEqual({
        key: 2,
        step: secondStep,
        location: {id: 2, parent_id: 1, parallel_group: 0, serial_group: 0},
        logLines: true,
        children: [],
      });
    });

    it("can build render data for parallel steps", function(){
      var renderableData;
      var firstStep = getFakeStep(2, 0, 1, 0, "");
      var secondStep = getFakeStep(3, 0, 1, 0, "");

      stepData = stepData.
        updateIn({id:2}, function(){return firstStep;}).
        updateIn({id:3}, function(){return secondStep;});

      renderableData = stepData.getRenderableData();
      expect(renderableData[1].aggregate).toBe(true);
      expect(renderableData[1].step).toEqual(firstStep);
      expect(renderableData[1].groupSteps[2]).toEqual({
        key: 2,
        step: firstStep,
        location: {id: 2, parent_id: 0, parallel_group: 1, serial_group: 0},
        logLines: true,
        children: [],
      });

      expect(renderableData[1].groupSteps[3]).toEqual({
        key: 3,
        step: secondStep,
        location: {id: 3, parent_id: 0, parallel_group: 1, serial_group: 0},
        logLines: true,
        children: [],
      });
    });

    it("can build render data for serial groups", function(){

      var renderableData;
      var firstStep = getFakeStep(2, 0, 0, 1, "");
      var secondStep = getFakeStep(3, 0, 0, 1, "");

      stepData = stepData.
        updateIn({id:2}, function(){return firstStep;}).
        updateIn({id:3}, function(){return secondStep;});

      renderableData = stepData.getRenderableData();
      expect(renderableData[1].serial).toBe(true);
      expect(renderableData[1].step).toEqual(firstStep);
      expect(renderableData[1].groupSteps[2]).toEqual({
        key: 2,
        step: firstStep,
        location: {id: 2, parent_id: 0, parallel_group: 0, serial_group: 1},
        logLines: true,
        children: [],
      });

      expect(renderableData[1].groupSteps[3]).toEqual({
        key: 3,
        step: secondStep,
        location: {id: 3, parent_id: 0, parallel_group: 0, serial_group: 1},
        logLines: true,
        children: [],
      });
    });

    it("can build render data for steps with a serial group inside a parallel group", function(){
      var renderableData;
      var firstStep = getFakeStep(2, 0, 1, 0, "");
      var secondStep = getFakeStep(4, 0, 1, 3, "");

      stepData = stepData.
        updateIn({id:2}, function(){return firstStep;}).
        updateIn({id:4}, function(){return secondStep;});

      renderableData = stepData.getRenderableData();
      expect(renderableData[1].aggregate).toBe(true);
      expect(renderableData[1].groupSteps[2]).toEqual({
        key: 2,
        step: firstStep,
        location: {id: 2, parent_id: 0, parallel_group: 1, serial_group: 0},
        logLines: true,
        children: [],
      });


      expect(renderableData[1].groupSteps[3].serial).toBe(true);
      expect(renderableData[1].groupSteps[3].groupSteps[4]).toEqual({
        key: 4,
        step: secondStep,
        location: {id: 4, parent_id: 0, parallel_group: 1, serial_group: 3},
        logLines: true,
        children: [],
      });
    });

    it("can build render data for steps with a parallel group inside a serial group", function(){
      var renderableData;
      var firstStep = getFakeStep(2, 0, 0, 1, "");
      var secondStep = getFakeStep(4, 0, 3, 1, "");

      stepData = stepData.
        updateIn({id:2}, function(){return firstStep;}).
        updateIn({id:4}, function(){return secondStep;});

      renderableData = stepData.getRenderableData();
      expect(renderableData[1].serial).toBe(true);
      expect(renderableData[1].groupSteps[2]).toEqual({
        key: 2,
        step: firstStep,
        location: {id: 2, parent_id: 0, parallel_group: 0, serial_group: 1},
        logLines: true,
        children: [],
      });


      expect(renderableData[1].groupSteps[3].aggregate).toBe(true);
      expect(renderableData[1].groupSteps[3].groupSteps[4]).toEqual({
        key: 4,
        step: secondStep,
        location: {id: 4, parent_id: 0, parallel_group: 3, serial_group: 1},
        logLines: true,
        children: [],
      });
    });

    it("can build render data for only serial groups in a parallel group", function(){
      var renderableData;
      var firstStep = getFakeStep(2, 0, 3, 1, "");
      var secondStep = getFakeStep(4, 0, 3, 1, "");

      stepData = stepData.
        updateIn({id:2}, function(){return firstStep;}).
        updateIn({id:4}, function(){return secondStep;});

      renderableData = stepData.getRenderableData();
      expect(renderableData[1].serial).toBe(true);
      expect(renderableData[1].groupSteps[2]).toBe(undefined);
      expect(renderableData[1].groupSteps[4]).toBe(undefined);

      expect(renderableData[1].groupSteps[3].groupSteps[2]).toEqual({
        key: 2,
        step: firstStep,
        location: {id: 2, parent_id: 0, parallel_group: 3, serial_group: 1},
        logLines: true,
        children: [],
      });

      expect(renderableData[1].groupSteps[3].groupSteps[4]).toEqual({
        key: 4,
        step: secondStep,
        location: {id: 4, parent_id: 0, parallel_group: 3, serial_group: 1},
        logLines: true,
        children: [],
      });
    });

    it("can build render data for only parallel groups in a serial group", function(){
      var renderableData;
      var firstStep = getFakeStep(2, 0, 1, 3, "");
      var secondStep = getFakeStep(4, 0, 1, 3, "");

      stepData = stepData.
        updateIn({id:2}, function(){return firstStep;}).
        updateIn({id:4}, function(){return secondStep;});

      renderableData = stepData.getRenderableData();
      expect(renderableData[1].aggregate).toBe(true);
      expect(renderableData[1].groupSteps[2]).toBe(undefined);
      expect(renderableData[1].groupSteps[4]).toBe(undefined);

      expect(renderableData[1].groupSteps[3].groupSteps[2]).toEqual({
        key: 2,
        step: firstStep,
        location: {id: 2, parent_id: 0, parallel_group: 1, serial_group: 3},
        logLines: true,
        children: [],
      });

      expect(renderableData[1].groupSteps[3].groupSteps[4]).toEqual({
        key: 4,
        step: secondStep,
        location: {id: 4, parent_id: 0, parallel_group: 1, serial_group: 3},
        logLines: true,
        children: [],
      });
    });

    it("can build render data for sequentel steps", function(){
      var renderableData;
      var firstStep = getFakeStep(1, 0, 0, 0, "");
      var secondStep = getFakeStep(2, 0, 0, 0, "");

      stepData = stepData.
        updateIn({id:1}, function(){return firstStep;}).
        updateIn({id:2}, function(){return secondStep;});

      renderableData = stepData.getRenderableData();
      expect(renderableData[1]).toEqual({
        key: 1,
        step: firstStep,
        location: {id: 1, parent_id: 0, parallel_group: 0, serial_group: 0},
        logLines: true,
        children: [],
      });
      expect(renderableData[2]).toEqual({
        key: 2,
        step: secondStep,
        location: {id: 2, parent_id: 0, parallel_group: 0, serial_group: 0},
        logLines: true,
        children: [],
      });
    });

    it("can build render data for nested parallel groups (AKA nested aggregates)", function(){
      var renderableData;
      var firstStep = getFakeStep(2, 0, 1, 0, "");
      var secondStep = getFakeStep(6, 0, 1, 0, "");
      var thirdStep = getFakeStep(4, 1, 3, 0, "");
      var fourthStep = getFakeStep(5, 1, 3, 0, "");

      stepData = stepData.
        updateIn({id:2}, function(){return firstStep;}).
        updateIn({id:6}, function(){return secondStep;}).
        updateIn({id:4}, function(){return thirdStep;}).
        updateIn({id:5}, function(){return fourthStep;});

      renderableData = stepData.getRenderableData();

      expect(renderableData[1].aggregate).toBe(true);
      expect(renderableData[1].step).toEqual(firstStep);
      expect(renderableData[1].groupSteps[2]).toEqual({
        key: 2,
        step: firstStep,
        location: {id: 2, parent_id: 0, parallel_group: 1, serial_group: 0},
        logLines: true,
        children: [],
      });

      expect(renderableData[1].groupSteps[6]).toEqual({
        key: 6,
        step: secondStep,
        location: {id: 6, parent_id: 0, parallel_group: 1, serial_group: 0},
        logLines: true,
        children: [],
      });

      var innerParallelGroup = renderableData[1].groupSteps[3];
      expect(innerParallelGroup.aggregate).toBe(true);
      expect(innerParallelGroup.step).toEqual(thirdStep);
      expect(innerParallelGroup.groupSteps[4]).toEqual({
        key: 4,
        step: thirdStep,
        location: {id: 4, parent_id: 1, parallel_group: 3, serial_group: 0},
        logLines: true,
        children: [],
      });
      expect(innerParallelGroup.groupSteps[5]).toEqual({
        key: 5,
        step: fourthStep,
        location: {id: 5, parent_id: 1, parallel_group: 3, serial_group: 0},
        logLines: true,
        children: [],
      });

    });

    it("can build render data for nested parallel groups", function(){
      var renderableData;
      var firstStep = getFakeStep(4, 1, 3, 0, "");
      var secondStep = getFakeStep(5, 1, 3, 0, "");

      stepData = stepData.updateIn({id: 4}, function(){return firstStep;});
      stepData = stepData.updateIn({id: 5}, function(){return secondStep;});
      renderableData = stepData.getRenderableData();

      expect(renderableData[1]).toBe(undefined);

      var thirdStep = getFakeStep(2, 0, 1, 0, "");
      stepData = stepData.updateIn({id: 2}, function(){return thirdStep;});
      renderableData = stepData.getRenderableData();

      expect(renderableData[1].hold).toBe(undefined);
      expect(renderableData[1].aggregate).toBe(true);
      expect(renderableData[1].groupSteps[2].step).toEqual(thirdStep);
      expect(renderableData[1].groupSteps[3].aggregate).toBe(true);
      expect(renderableData[1].groupSteps[3].groupSteps[4].step).toEqual(firstStep);
      expect(renderableData[1].groupSteps[3].groupSteps[5].step).toEqual(secondStep);
    });

    it("can build render data with hooks that are aggregates", function(){
      var renderableData;
      var firstStep = getFakeStep(1, 0, 0, 0, "");
      var secondStep = getFakeStep(3, 1, 2, 0, "success");
      var thirdStep = getFakeStep(4, 1, 2, 0, "success");

      stepData = stepData.
        updateIn({id:1}, function(){return firstStep;}).
        updateIn({id:3}, function(){return secondStep;}).
        updateIn({id:4}, function(){return thirdStep;});

      renderableData = stepData.getRenderableData();
      expect(renderableData[1].step).toEqual(firstStep);
      expect(renderableData[1].children[2].aggregate).toBe(true);
      expect(renderableData[1].children[2].groupSteps[3].step).toEqual(secondStep);
      expect(renderableData[1].children[2].groupSteps[4].step).toEqual(thirdStep);
    });
  });

  describe("#translateLocation", function(){
    it("ignores locations that are already of the new type", function(){
      expect(stepData.translateLocation({id:2, parallel_group: 1, parent_id:7}, 3, false)).toEqual({id:2, parallel_group: 1, parent_id:7});
    });

    it("can take an array of ints with an id and produce a location of the new type", function(){
      expect(stepData.translateLocation([0], false)).toEqual({id:1, parallel_group: 0, parent_id:0});

      expect(stepData.translateLocation([1,0], false)).toEqual({id:3, parallel_group: 2, parent_id:0});
      expect(stepData.translateLocation([1,1], false)).toEqual({id:4, parallel_group: 2, parent_id:0});

      expect(stepData.translateLocation([2,0], false)).toEqual({id:6, parallel_group: 5, parent_id:0});
      expect(stepData.translateLocation([2,1], false)).toEqual({id:7, parallel_group: 5, parent_id:0});

      expect(stepData.translateLocation([2,2,0], false)).toEqual({id:9, parallel_group: 8, parent_id:5});
      expect(stepData.translateLocation([2,2,1], false)).toEqual({id:10, parallel_group: 8, parent_id:5});

      expect(stepData.translateLocation([2,3], false)).toEqual({id:11, parallel_group: 5, parent_id:0});

      expect(stepData.translateLocation([2,2,2,0], false)).toEqual({id:13, parallel_group: 12, parent_id:8});
      expect(stepData.translateLocation([2,2,2,1], false)).toEqual({id:14, parallel_group: 12, parent_id:8});
      expect(stepData.translateLocation([2,2,2,2], false)).toEqual({id:15, parallel_group: 12, parent_id:8});

      expect(stepData.translateLocation([2,2,2,3,0], false)).toEqual({id:17, parallel_group: 16, parent_id:12});
      expect(stepData.translateLocation([2,2,2,3,1], false)).toEqual({id:18, parallel_group: 16, parent_id:12});


      expect(stepData.translateLocation([3,0], false)).toEqual({id:20, parallel_group: 19, parent_id:0});
      expect(stepData.translateLocation([3,1], false)).toEqual({id:21, parallel_group: 19, parent_id:0});
      expect(stepData.translateLocation([3,2], false)).toEqual({id:22, parallel_group: 19, parent_id:0});


      expect(stepData.translateLocation([4], false)).toEqual({id:23, parallel_group: 0, parent_id:0});

      // if lots of events are being streamed in aggregate, we may get a child before we get a parent
      expect(stepData.translateLocation([5,1,0], false)).toEqual({id:25, parallel_group: 24, parent_id:0});

      expect(stepData.translateLocation([6,0,0], false)).toEqual({id:27, parallel_group: 26, parent_id:0});
      expect(stepData.translateLocation([6,0,1], false)).toEqual({id:28, parallel_group: 26, parent_id:0});

      expect(stepData.translateLocation([6,1], false)).toEqual({id:30, parallel_group: 29, parent_id:0});

      expect(stepData.translateLocation([6,2,0], false)).toEqual({id:32, parallel_group: 31, parent_id:29});
      expect(stepData.translateLocation([6,2,1], false)).toEqual({id:33, parallel_group: 31, parent_id:29});


      expect(stepData.translateLocation([7], false)).toEqual({id:34, parallel_group: 0, parent_id:0});
      expect(stepData.translateLocation([8], true)).toEqual({id:35, parallel_group: 0, parent_id:34});

      expect(stepData.translateLocation([9,0], false)).toEqual({id:37, parallel_group: 36, parent_id:0});
      expect(stepData.translateLocation([9,1], true)).toEqual({id:38, parallel_group: 36, parent_id:37});
    });
  });
});
