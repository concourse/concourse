If you need to create a rootfs tarball in s3 for testflight test cases, do the following:

Run `DOCKER_IMAGE=concourse/time-resource VERSION=0.0.1 fly -t lite execute -c build-rootfs.yml -o output=/tmp/testflight`

Once you have the tarball, you can upload to s3. **NOTE**: This script will make the bucket publically readable, so if you don't want that on an existing bucket, give a non-existent bucket name and it will be created. To do so, run

```sh
./setup-external-dependencies BUCKET_NAME /path/to/rootfs.tar.gz
```
