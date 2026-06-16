# Baggageclaim

*A volume manager for Concourse containers*

![Baggage Claim](https://farm4.staticflickr.com/3365/4623535134_c88f474f8d_d.jpg)

[by](https://creativecommons.org/licenses/by-nc-nd/2.0/) [atmx](https://www.flickr.com/photos/atmtx/)

## About

*Baggageclaim* allows you to create and layer volumes on a remote server. This
is particularly useful when used with [bind mounts][bind-mounts] or the RootFS
when creating containers. It allows directories to be made which can be
populated before having copy-on-write (CoW) layers layered on top. This allows
multiple containers to read and write to the same underlying volume without
colliding with each others work, since they'll write to different CoW layers.

Baggageclaim runs as an HTTP server. There is no authentication, but is
configured to only listen on localhost (by default). A Concourse worker forwards
a connection to the Baggageclaim server over SSH to a Concourse web node,
allowing the web node to send requests to Baggageclaim securely.

[bind-mounts]: http://man7.org/linux/man-pages/man8/mount.8.html#COMMAND-LINE_OPTIONS
