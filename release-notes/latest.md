#### <sub><sup><a name="5305" href="#5305">:link:</a></sup></sub> feature

* We've updated the way that hijacked containers get garbage collected

  We are no longer relying on garden to clean up hijacked containers. Instead, we have implemented this functionality in concourse itself. This makes it much more portable to different container backends.


