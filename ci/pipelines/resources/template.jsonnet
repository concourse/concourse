local resource = std.extVar("resource");

local build_params =
  if resource == "registry-image" then
    {
      build: resource+"-resource",
      build_args: {
        PRIVATE_REPO: "((registry_image_resource.private_repo))",
        PRIVATE_REPO_USERNAME: "((registry_image_resource_private_repo.username))",
        PRIVATE_REPO_PASSWORD: "((registry_image_resource_private_repo.password))",
      }
    }
  else if resource == "semver" then
    {
      build: resource+"-resource",
      build_args: {
        SEMVER_TESTING_ACCESS_KEY_ID: "((semver_resource_bucket.access_key))",
        SEMVER_TESTING_BUCKET: "((semver_resource.bucket))",
        SEMVER_TESTING_REGION: "((semver_resource.region))",
        SEMVER_TESTING_SECRET_ACCESS_KEY: "((semver_resource_bucket.secret_key))"
      }
    }
  else if resource == "s3" then
    {
      build: resource+"-resource",
      build_args: {
        S3_TESTING_ACCESS_KEY_ID: "((s3_resource_bucket.access_key))",
        S3_TESTING_SECRET_ACCESS_KEY: "((s3_resource_bucket.secret_key))",
        S3_TESTING_BUCKET: "((s3_resource.bucket))",
        S3_VERSIONED_TESTING_BUCKET: "((s3_resource.versioned_bucket))",
        S3_TESTING_REGION: "((s3_resource.region))",
        S3_ENDPOINT: "https://s3.amazonaws.com"
      }
    }
  else if resource == "concourse-pipeline" then
    {
      build: ".",
      dockerfile: resource+"-resource/Dockerfile"
    }
  else
    {
      build: resource+"-resource"
    };

local extra_resources =
  if resource == "cf" then
    [
      {
        name: "cf-cli",
        type: "s3",
        source: {
          bucket: "cf-cli-releases",
          regexp: "releases/v([\\d\\.]+)/cf-cli_.*_linux_x86-64.tgz",
          region_name: "us-west-1"
        }
      }
    ]
  else if resource == "concourse-pipeline" then
    [
      {
        name: "fly",
        type: "github-release",
        source: {
          user: "concourse",
          repository: "concourse",
          access_token: "((concourse_github_dummy.access_token))"
        }
      }
    ]
  else
    [];

local extra_gets =
  if resource == "cf" then
    [
      {
        get: "cf-cli",
        trigger: true,
        params: {globs: ["cf-cli*linux*"]}
      }
    ]
  else if resource == "concourse-pipeline" then
    [
      {
        get: "fly",
        params: {globs: ["fly_linux_amd64"]}
      }
    ]
  else
    [];

local create_release = {
  platform: "linux",
  image_resource: {
    type: "registry-image",
    source: {repository: "ubuntu"}
  },
  inputs: [
    {name: "version"},
    {name: "resource-image-dev"}
  ],
  outputs: [
    {name: "release"},
    {name: "docker"}
  ],
  run: {
    path: "bash",
    args: [
      "-exc",
      |||
        cat <<EOF > resource-image-dev/resource_metadata.json
        {
          "type": "%(resource)s",
          "version": "$(cat version/number)",
          "privileged": %(privileged)s
        }
        EOF

        version="$(cat version/number)"
        echo "v${version}" > release/name

        echo $version | cut -d. -f1      > docker/tags
        echo $version | cut -d. -f1,2   >> docker/tags
        echo $version | cut -d. -f1,2,3 >> docker/tags

        cd resource-image-dev
        tar -czf rootfs.tgz -C rootfs .
        tar -czf ../release/%(resource)s-resource-${version}.tgz rootfs.tgz resource_metadata.json
      ||| % {
        resource: resource,
        privileged: resource == "docker-image"
      }
    ]
  }
};

local publish_job(bump) = {
  name: bump,
  plan: [
    {
      aggregate: [
        {
          get: "resource-repo",
          passed: ["build"]
        },
        {
          get: "resource-image-dev",
          passed: ["build"],
          params: {save: true}
        },
        {
          get: "version",
          params: {bump: bump}
        }
      ]
    },
    {
      task: "create-release",
      config: create_release
    },
    {
      aggregate: [
        {
          put: "resource-image-latest",
          params: {
            load: "resource-image-dev",
            additional_tags: "docker/tags"
          }
        },
        {
          put: "resource-repo-release",
          params: {
            name: "release/name",
            commitish: "resource-repo/.git/ref",
            tag: "version/version",
            tag_prefix: "v",
            globs: ["release/*.tgz"]
          }
        }
      ]
    },
    {
      put: "version",
      params: {file: "version/version"}
    }
  ]
};

{
  resource_types: [
    {
      name: "pull-request",
      type: "registry-image",
      source: {repository: "jtarchie/pr"}
    },
    {
      name: "semver",
      type: "registry-image",
      source: {repository: "concourse/semver-resource"}
    },
    {
      name: "docker-image",
      type: "registry-image",
      source: {repository: "concourse/docker-image-resource"},
      privileged: true
    },
    {
      name: "github-release",
      type: "registry-image",
      source: {repository: "concourse/github-release-resource"}
    }
  ],
  resources: [
    {
      name: "alpine-edge",
      type: "docker-image",
      source: {
        repository: "alpine",
        tag: "edge"
      }
    },
    {
      name: "resource-repo",
      type: "git",
      source: {
        uri: "git@github.com:concourse/"+resource+"-resource",
        branch: "master",
        private_key: "((concourse_bot_private_key))"
      }
    },
    {
      name: "resource-repo-release",
      type: "github-release",
      source: {
        owner: "concourse",
        repository: resource+"-resource",
        access_token: "((concourse_bot_access_token))"
      }
    },
    {
      name: "version",
      type: "semver",
      source: {
        driver: "git",
        uri: "git@github.com:concourse/"+resource+"-resource",
        branch: "version",
        file: "version",
        private_key: "((concourse_bot_private_key))"
      }
    },
    {
      name: "resource-pr",
      type: "pull-request",
      source: {
        repo: "concourse/"+resource+"-resource",
        base: "master",
        access_token: "((pull_requests_access_token))",
        [if resource == "s3" || resource == "semver" then "label"]: "approved-for-ci"
      }
    },
    {
      name: "resource-image-latest",
      type: "docker-image",
      source: {
        repository: "concourse/"+resource+"-resource",
        tag: "latest",
        username: "((docker.username))",
        password: "((docker.password))"
      }
    },
    {
      name: "resource-image-dev",
      type: "docker-image",
      source: {
        repository: "concourse/"+resource+"-resource",
        tag: "dev",
        username: "((docker.username))",
        password: "((docker.password))"
      }
    }
  ] + extra_resources,
  jobs: [
    {
      name: "build",
      plan: [
        {
          aggregate: [
            {
              get: resource+"-resource",
              resource: "resource-repo",
              trigger: true
            },
            {
              get: "alpine-edge",
              params: {save: true},
              trigger: true
            },
          ] + extra_gets,
        },
        {
          put: "resource-image-dev",
          params: {
            load_base: "alpine-edge"
          } + build_params
        }
      ]
    },
    {
      name: "prs",
      serial: true,
      public: true,
      plan: [
        {
          aggregate: [
            {
              get: "resource-pr",
              trigger: true,
              version: "every"
            },
            {
              get: "alpine-edge",
              params: {save: true},
              trigger: true
            },
          ] + extra_gets
        },
        {
          put: resource+"-resource",
          resource: "resource-pr",
          params: {
            path: "resource-pr",
            status: "pending"
          },
          get_params: {fetch_merge: true}
        },
        {
          put: "resource-image-dev",
          params: {
            load_base: "alpine-edge",
            tag: resource+"-resource/.git/id",
            tag_prefix: "pr-",
          } + build_params,
          on_failure: {
            put: "resource-pr",
            params: {
              path: "resource-pr",
              status: "failure"
            }
          },
          on_success: {
            put: "resource-pr",
            params: {
              path: "resource-pr",
              status: "success"
            }
          }
        }
      ]
    },
    publish_job("major"),
    publish_job("minor"),
    publish_job("patch")
  ]
}
