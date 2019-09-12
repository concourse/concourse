#### <sub><sup><a name="note-cred-redacting" href="#note-cred-redacting">:link:</a></sup></sub> fix

* Credentials fetched from a credential manager will now be automatically redacted from build output, thanks to a couple of PRs by @evanchaoli! #4311

  Please don't rely on this functionality to keep your secrets safe: you should continue to prevent accidental credential leakage, and only treat this as a safety net.

  > NOTE: In its current form, credentials that get 'split' into multiple `write()` calls will not be redacted. This may happen for large credentials, or if you're just unlucky. Work to improve this is in-progress: #4398

#### <sub><sup><a name="note-cluster-log" href="#note-cluster-log">:link:</a></sup></sub> fix

* The cluster name can now be added to each and every log line with the handy dandy `--log-cluster-name` flag, available on the `web` nodes. This can be used in a scenario where you have multiple Concourse clusters forwarding logs to a common sink and have no other way of categorizing the logs. Thanks again @evanchaoli! #4387

#### <sub><sup><a name="note-version-string" href="#note-version-string">:link:</a></sup></sub> fix

* To pin a version for a resource on `get` step, the `set-pipeline` command for fly will now take only string value in key-value pair of given version. #4371
