#### <sub><sup><a name="5305" href="#5305">:link:</a></sup></sub> feature

* We've updated the way that hijacked containers get garbage collected

  We are no longer relying on garden to clean up hijacked containers. Instead, we have implemented this functionality in concourse itself. This makes it much more portable to different container backends.


#### <sub><sup><a name="5397" href="#5397">:link:</a></sup></sub> feature, breaking

* @pnsantos updated the Material Design icon library version to `5.0.45`.

  **note:** some icons changed names (e.g. `mdi-github-circle` was changed to `mdi-github`) so after this update you might have to update some `icon:` references
