# Security Policy

## Reporting a Vulnerability

To report a security issue, please send an email to <security@concourse-ci.org>. Thank you!

Only the latest release gets security patches.

Once we receive a report the following will happen:

1. We will confirm receipt of the report.
2. We will follow-up with our assessment and timeline for resolution if applicable.

## EOL Support

If you're looking for security patches for an EOL version of Concourse, please reach out to [Pixel Air IO](https://pixelair.io/).

## Certificate trust updates

Build `concourse` and `fly` with Go 1.27.3 or newer to reject the compromised
eMudhra intermediate certificate chains
([GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809)). Existing binaries must be
replaced with rebuilt releases, and web and worker processes restarted.

The root Dockerfile updates the local development image's Wolfi CA packages.
Official release images are built separately in
[concourse/concourse-docker](https://github.com/concourse/concourse-docker);
their OS trust stores and any bundled resource images and prebuilt tools need
the corresponding upstream updates and rebuilds as well.
