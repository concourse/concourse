local resource = std.extVar("resource");

local build_params =
  if resource == "registry-image" then
    {
      build: resource+"-resource",
      build_args: {
        DOCKER_PRIVATE_REPO: "((registry_image_resource_docker.private_repo))",
        DOCKER_PRIVATE_USERNAME: "((registry_image_resource_docker.username))",
        DOCKER_PRIVATE_PASSWORD: "((registry_image_resource_docker.password))",
        DOCKER_PUSH_REPO: "((registry_image_resource_docker.push_repo))",
        DOCKER_PUSH_USERNAME: "((registry_image_resource_docker.username))",
        DOCKER_PUSH_PASSWORD: "((registry_image_resource_docker.password))",
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
        params: {globs: ["fly-*-linux-amd64.tgz"]}
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
    {name: "resource-image-dev-alpine"},
    {name: "resource-image-dev-ubuntu"}
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
        cat <<EOF > resource_metadata.json
        {
          "type": "%(resource)s",
          "version": "$(cat version/number)",
          "privileged": %(privileged)s,
          "unique_version_history": %(unique_version_history)s
        }
        EOF

        version="$(cat version/number)"
        echo "v${version}" > release/name

        echo $version | cut -d. -f1      > docker/tags
        echo $version | cut -d. -f1,2   >> docker/tags
        echo $version | cut -d. -f1,2,3 >> docker/tags

        pushd resource-image-dev-alpine
          cp ../resource_metadata.json .
          tar -czf rootfs.tgz -C rootfs .
          tar -czf ../release/%(resource)s-resource-${version}-alpine.tgz rootfs.tgz resource_metadata.json
        popd

        pushd resource-image-dev-ubuntu
          cp ../resource_metadata.json .
          tar -czf rootfs.tgz -C rootfs .
          tar -czf ../release/%(resource)s-resource-${version}-ubuntu.tgz rootfs.tgz resource_metadata.json
        popd
      ||| % {
        resource: resource,
        privileged: resource == "docker-image",
        unique_version_history: resource == "time"
      }
    ]
  }
};
local generate_dpkg_list = {
  platform: "linux",
  inputs: [
    { name: "version" },
  ],
  outputs: [
    { name: "dpkg-file" },
  ],
  run: {
    path: "bash",
    args: [
      "-exc",
      |||
        VERSION="$(cat version/number)"
        RESOURCE="%(resource)s"
        DPKG_FILE="${RESOURCE}-dpkg-list-${VERSION}.txt"
        dpkg -l > "dpkg-file/${DPKG_FILE}"
      ||| % {
        resource: resource,
      },
    ],
  },
};

local publish_job(bump) = {
  name: "publish-" + bump,
  plan: [
    {
      in_parallel: [
        {
          get: "resource-repo",
          passed: ["build-alpine", "build-ubuntu"]
        },
        {
          get: "resource-image-dev-alpine",
          passed: ["build-alpine"],
          params: {save: true}
        },
        {
          get: "resource-image-dev-ubuntu",
          passed: ["build-ubuntu"],
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
      task: "generate-dpkg-list",
      config: generate_dpkg_list,
      image: "resource-image-dev-ubuntu",
    },
    {
      in_parallel: [
        {
          put: "resource-image-alpine",
          params: {
            load: "resource-image-dev-alpine",
            additional_tags: "docker/tags",
            tag_as_latest: true
          }
        },
        {
          put: "resource-image-ubuntu",
          params: {
            load: "resource-image-dev-ubuntu",
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
      put: "dpkg-list-store",
      params: {
        file: "dpkg-file/" + resource + "-dpkg-list-*.txt",
      },
    },
    {
      put: "version",
      params: {file: "version/version"}
    }
  ]
};

local determine_base(distro) =
  if distro == "alpine" then
    "alpine-edge"
  else if distro == "ubuntu" then
    "ubuntu-bionic";

local validate_pr(distro) = {
  name: "prs-" + distro,
  serial: true,
  public: true,
  plan: [
    {
      in_parallel: [
        {
          get: "resource-pr",
          trigger: true,
          version: "every"
        },
        {
          get: determine_base(distro),
          params: {save: true},
          trigger: true
        }
      ] + extra_gets
    },
    {
      put: resource+"-resource",
      resource: "resource-pr",
      params: {
        path: "resource-pr",
        context: "status-" + distro,
        status: "pending"
      },
      get_params: {fetch_merge: true}
    },
    {
      put: "resource-image-dev-" + distro,
      params: {
        load_base: determine_base(distro),
        tag: resource+"-resource/.git/id",
        tag_prefix: "pr-" + distro + "-",
        dockerfile: resource+"-resource/dockerfiles/" + distro + "/Dockerfile",
      } + build_params,
      on_failure: {
        put: "resource-pr",
        params: {
          path: "resource-pr",
          context: "status-" + distro,
          status: "failure"
        }
      },
      on_success: {
        put: "resource-pr",
        params: {
          path: "resource-pr",
          context: "status-" + distro,
          status: "success"
        }
      }
    }
  ]
};

local build_image(distro) = {
  name: "build-" + distro,
  plan: [
    {
      in_parallel: [
        {
          get: resource+"-resource",
          resource: "resource-repo",
          trigger: true
        },
        {
          get: determine_base(distro),
          params: {save: true},
          trigger: true
        },
      ] + extra_gets,
    },
    {
      put: "resource-image-dev-" + distro,
      params: {
        load_base: determine_base(distro),
        dockerfile: resource+"-resource/dockerfiles/" + distro + "/Dockerfile",
      } + build_params
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
    },
    {
      name: "s3",
      type: "registry-image",
      source: {repository: "concourse/s3-resource"}
    },
    {
      name: "gcs",
      type: "registry-image",
      source: {repository: "frodenas/gcs-resource"}
    }
  ],
  resources: [
    {
      name: "alpine-edge",
      type: "docker-image",
      icon: "docker",
      source: {
        repository: "alpine",
        tag: "edge"
      }
    },
    {
      name: "ubuntu-bionic",
      type: "docker-image",
      icon: "docker",
      source: {
        repository: "ubuntu",
        tag: "bionic"
      }
    },
    {
      name: "resource-repo",
      type: "git",
      icon: "github-circle",
      source: {
        uri: "git@github.com:concourse/"+resource+"-resource",
        branch: "master",
        private_key: "((concourse_bot_private_key))"
      }
    },
    {
      name: "resource-repo-release",
      type: "github-release",
      icon: "package-variant-closed",
      source: {
        owner: "concourse",
        repository: resource+"-resource",
        access_token: "((concourse_bot_access_token))"
      }
    },
    {
      name: "version",
      type: "semver",
      icon: "tag",
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
      icon: "source-pull",
      source: {
        repo: "concourse/"+resource+"-resource",
        base: "master",
        access_token: "((pull_requests_access_token))",
        [if resource == "s3" || resource == "semver" then "label"]: "approved-for-ci"
      }
    },
    {
      name: "resource-image-alpine",
      type: "docker-image",
      icon: "docker",
      source: {
        repository: "concourse/"+resource+"-resource",
        tag: "alpine",
        username: "((docker.username))",
        password: "((docker.password))"
      }
    },
    {
      name: "resource-image-ubuntu",
      type: "docker-image",
      icon: "docker",
      source: {
        repository: "concourse/"+resource+"-resource",
        tag: "ubuntu",
        username: "((docker.username))",
        password: "((docker.password))"
      }
    },
    {
      name: "resource-image-dev-alpine",
      type: "docker-image",
      icon: "docker",
      source: {
        repository: "concourse/"+resource+"-resource",
        tag: "dev",
        username: "((docker.username))",
        password: "((docker.password))"
      }
    },
    {
      name: "resource-image-dev-ubuntu",
      type: "docker-image",
      icon: "docker",
      source: {
        repository: "concourse/"+resource+"-resource",
        tag: "dev-ubuntu",
        username: "((docker.username))",
        password: "((docker.password))"
      }
    },
    {
      name: "dpkg-list-store",
      type: "gcs",
      source: {
        bucket: "concourse-ubuntu-dpkg-list",
        json_key: "((concourse_dpkg_list_json_key))",
        regexp: resource + "-dpkg-list-(.*).txt",
      },
    },
  ] + extra_resources,
  jobs: [
    build_image("alpine"),
    build_image("ubuntu"),
    validate_pr("alpine"),
    validate_pr("ubuntu"),
    publish_job("major"),
    publish_job("minor"),
    publish_job("patch")
  ]
}
