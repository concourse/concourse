#### <sub><sup><a name="note-lidar" href="#note-lidar">:link:</a></sup></sub> feature

* Resource checking has changed, but (hopefully) you won't notice!

The entire system has been redesigned to be asynchronous, but that shouldn't have any affect on your existing workflows. `fly check-resource` and `fly check-resource-type` will continue to work the way you expect them to, however you can now specify an `--async` flag if you don't want to wait for the check to finish.

If you're interested in more detail about what changed you can have a look at the corresponding PR #4202 or the initial issue #3788

It's worth noting that concourse performs a lot of checks (like A LOT). This means the checks table will tend to grow very quickly. By default checks get gc'ed every 6 hrs, but this interval can be configured by specifying a `CONCOURSE_GC_CHECK_RECYCLE_PERIOD`. If you want to reduce the number of checks that happen, you can start making heavier use of the `webhook` endpoints to trigger checks from an external source. This allows you to significantly reduce the `check_every` interval (default 1m) for your resource without impacting the time it takes to schedule a build. 

This feature is off by default but can be turned on via the `CONCOURSE_ENABLE_LIDAR` flag.

#### <sub><sup><a name="note-cred-redacting" href="#note-cred-redacting">:link:</a></sup></sub> fix

* Credentials fetched from a credential manager will now be automatically redacted from build output, thanks to a couple of PRs by @evanchaoli! #4311

  Please don't rely on this functionality to keep your secrets safe: you should continue to prevent accidental credential leakage, and only treat this as a safety net.

  > NOTE: In its current form, credentials that get 'split' into multiple `write()` calls will not be redacted. This may happen for large credentials, or if you're just unlucky. Work to improve this is in-progress: #4398

#### <sub><sup><a name="note-cluster-log" href="#note-cluster-log">:link:</a></sup></sub> fix

* The cluster name can now be added to each and every log line with the handy dandy `--log-cluster-name` flag, available on the `web` nodes. This can be used in a scenario where you have multiple Concourse clusters forwarding logs to a common sink and have no other way of categorizing the logs. Thanks again @evanchaoli! #4387

