# baggage claim

*a volume manager for garden containers*

![Baggage Claim](https://farm4.staticflickr.com/3365/4623535134_c88f474f8d_d.jpg)

[by](https://creativecommons.org/licenses/by-nc-nd/2.0/) [atmx](https://www.flickr.com/photos/atmtx/)

## reporting issues and requesting features

please report all issues and feature requests in [concourse/concourse](https://github.com/concourse/concourse/issues)

## about

*baggageclaim* allows you to create and layer volumes on a remote server. This
is particularly useful when used with [bind mounts][bind-mounts] or the RootFS
when using [Garden][garden]. It allows directories to be made which can be
populated before having copy-on-write layers layered on top. e.g. to provide
caching.

[bind-mounts]: http://man7.org/linux/man-pages/man8/mount.8.html#COMMAND-LINE_OPTIONS
[garden]: https://github.com/cloudfoundry-incubator/garden-linux
