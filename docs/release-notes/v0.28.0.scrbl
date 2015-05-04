#lang concourse/docs

@title[#:style '(quiet unnumbered)]{v0.28.0}

Removes the Turbine component. See v0.27.0's release notes.

Note that the deployment manifest has changed once again, this time to remove
the @code{turbine} job. See the @hyperlink["https://github.com/concourse/concourse/tree/2959b6cee2178421fd14cf0150798dbe66ccc9b1/manifests"]{example manifests}.

If you skip v0.27.0, any builds running @emph{during} the Concourse upgrade will
be orphaned. If you're upgrading from v0.27.0 everything should be fine.
