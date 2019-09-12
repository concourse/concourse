#### <sub><sup><a name="note-lidar" href="#note-lidar">:link:</a></sup></sub> feature

* Resource checking has changed, but hopefully you won't notice! #4202

The responsibility of checking resources is now split between two components: `scanner` and `checker`. The `scanner` will iterate through every resource (and its corresponding type) and add entries to the `checks` table. The `checker` will then run any checks that end up there. If you do a `fly check-resource` or `fly check-resource-type`, we skip the `scanner` component and insert a check directly into the database.

This feature can be turned on/off through the `CONCOURSE_ENABLE_LIDAR` flag. However, this flag will only toggle the `scanner` mechanism. All API requests will be tracked in the checks table regardless of whether or not this feature is enabled, which means the `checker` will always be running. 

It's worth noting that concourse performs A LOT of checks (like a lot a lot). This means the checks table will tend to grow very quickly. Be default checks get gc'ed every 6 hrs, but this interval can be configured by specifying a `CONCOURSE_GC_CHECK_RECYCLE_PERIOD`. Moving forward if you want to reduce the number of checks that happen, you can start making heavier use of the `webhook` endpoints to trigger checks from an external source. This allows you to significantly reduce the `check_every` interval (default 1m) for your resource without impacting the time it takes to schedule a build. 

#### <sub><sup><a name="note-cred-redacting" href="#note-cred-redacting">:link:</a></sup></sub> fix

* Credentials fetched from a credential manager will now be automatically redacted from build output, thanks to a couple of PRs by @evanchaoli! #4311

  Please don't rely on this functionality to keep your secrets safe: you should continue to prevent accidental credential leakage, and only treat this as a safety net.

  > NOTE: In its current form, credentials that get 'split' into multiple `write()` calls will not be redacted. This may happen for large credentials, or if you're just unlucky. Work to improve this is in-progress: #4398

#### <sub><sup><a name="note-cluster-log" href="#note-cluster-log">:link:</a></sup></sub> fix

* The cluster name can now be added to each and every log line with the handy dandy `--log-cluster-name` flag, available on the `web` nodes. This can be used in a scenario where you have multiple Concourse clusters forwarding logs to a common sink and have no other way of categorizing the logs. Thanks again @evanchaoli! #4387

