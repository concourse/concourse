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
