# Spin up a Concourse

One ATC, one worker, pointing at local Postgres's `atc` database:

```sh
sudo ./concourse --user pilot
```

Must run as root for Garden; `--user` is which user to run unprivileged
processes (like ATC) as.
