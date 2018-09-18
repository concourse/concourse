package algorithm_test

import (
	. "github.com/onsi/ginkgo/extensions/table"
)

var _ = DescribeTable("Input resolving",
	(Example).Run,

	Entry("bosh memory leak regression test", Example{
		LoadDB: "testdata/bosh-versions.json.gz",

		Inputs: Inputs{
			{
				Name:     "bosh-src",
				Resource: "bosh-src",
				Passed: []string{
					"unit-1.9",
					"unit-2.1",
					"integration-2.1-mysql",
					"integration-1.9-postgres",
					"integration-2.1-postgres",
				},
			},
			{
				Name:     "bosh-load-tests",
				Resource: "bosh-load-tests",
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"bosh-src":        "imported-r88v9814",
				"bosh-load-tests": "imported-r89v7204",
			},
		},
	}),

	Entry("concourse deploy high cpu regression test", Example{
		LoadDB: "testdata/concourse-versions-high-cpu-deploy.json.gz",

		Inputs: Inputs{
			{
				Name:     "concourse",
				Resource: "concourse",
				Passed: []string{
					"testflight",
					"bin-testflight",
				},
			},
			{
				Name:     "version",
				Resource: "version",
				Passed: []string{
					"testflight",
					"bin-testflight",
				},
			},
			{
				Name:     "candidate-release",
				Resource: "candidate-release",
				Passed: []string{
					"testflight",
				},
			},
			{
				Name:     "garden-linux-release",
				Resource: "garden-linux",
				Passed: []string{
					"testflight",
				},
			},
			{
				Name:     "bin-rc",
				Resource: "bin-rc",
				Passed: []string{
					"bin-testflight",
				},
			},
			{
				Name:     "bosh-stemcell",
				Resource: "aws-stemcell",
			},
			{
				Name:     "deployments",
				Resource: "deployments",
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"candidate-release":    "imported-r238v448886",
				"deployments":          "imported-r45v448469",
				"bosh-stemcell":        "imported-r48v443997",
				"bin-rc":               "imported-r765v448889",
				"garden-linux-release": "imported-r17v443811",
				"version":              "imported-r12v448884",
				"concourse":            "imported-r62v448881",
			},
		},
	}),

	Entry("relint rc disabled stemcell high cpu regression test", Example{
		LoadDB: "testdata/relint-versions.json.gz",

		Inputs: Inputs{
			{
				Name:     "runtime-ci",
				Resource: "runtime-ci",
			},
			{
				Name:     "diego-cf-compatibility",
				Resource: "diego-cf-compatibility",
			},
			{
				Name:     "cf-release",
				Resource: "cf-release-develop",
				Passed: []string{
					"bosh-lite-acceptance-tests",
					"a1-diego-cats",
				},
			},
			{
				Name:     "bosh-lite-stemcell",
				Resource: "bosh-lite-stemcell",
				Passed: []string{
					"bosh-lite-acceptance-tests",
				},
			},
			{
				Name:     "stemcell",
				Resource: "aws-stemcell",
				Passed: []string{
					"a1-diego-cats",
				},
			},
			{
				Name:     "diego-final-releases",
				Resource: "diego-final-releases",
				Passed: []string{
					"a1-diego-cats",
					"bosh-lite-acceptance-tests",
				},
			},
			{
				Name:     "diego-release-master",
				Resource: "diego-release-master",
				Passed: []string{
					"a1-diego-cats",
					"bosh-lite-acceptance-tests",
				},
			},
			{
				Name:     "garden-linux-release-tarball",
				Resource: "garden-linux-release-tarball",
				Passed: []string{
					"a1-diego-cats",
					"bosh-lite-acceptance-tests",
				},
			},
			{
				Name:     "etcd-release-tarball",
				Resource: "etcd-release-tarball",
				Passed: []string{
					"a1-diego-cats",
					"bosh-lite-acceptance-tests",
				},
			},
			{
				Name:     "cflinuxfs2-rootfs-release-tarball",
				Resource: "cflinuxfs2-rootfs-release-tarball",
				Passed: []string{
					"a1-diego-cats",
					"bosh-lite-acceptance-tests",
				},
			},
		},

		Result: Result{
			OK:     false,
			Values: map[string]string{},
		},
	}),

	Entry("relint rc high cpu regression test", Example{
		LoadDB: "testdata/relint-versions-2.json.gz",

		Inputs: Inputs{
			{
				Name:     "runtime-ci",
				Resource: "runtime-ci",
			},
			{
				Name:     "diego-cf-compatibility",
				Resource: "diego-cf-compatibility",
			},
			{
				Name:     "cf-release",
				Resource: "cf-release-develop",
				Passed: []string{
					"bosh-lite-acceptance-tests",
					"aws-acceptance-tests",
					"vsphere-acceptance-tests",
				},
			},
			{
				Name:     "bosh-lite",
				Resource: "bosh-lite",
				Passed: []string{
					"bosh-lite-acceptance-tests",
				},
			},
			{
				Name:     "bosh-lite-stemcell",
				Resource: "bosh-lite-stemcell",
				Passed: []string{
					"bosh-lite-acceptance-tests",
				},
			},
			{
				Name:     "vsphere-stemcell",
				Resource: "vsphere-stemcell",
				Passed: []string{
					"vsphere-acceptance-tests",
				},
			},
			{
				Name:     "aws-stemcell",
				Resource: "aws-stemcell",
				Passed: []string{
					"aws-acceptance-tests",
				},
			},
			{
				Name:     "diego-release-tarball",
				Resource: "diego-release-tarball",
				Passed: []string{
					"aws-acceptance-tests",
					"bosh-lite-acceptance-tests",
					"vsphere-acceptance-tests",
				},
			},
			{
				Name:     "garden-linux-release-tarball",
				Resource: "garden-linux-release-tarball",
				Passed: []string{
					"aws-acceptance-tests",
					"bosh-lite-acceptance-tests",
					"vsphere-acceptance-tests",
				},
			},
			{
				Name:     "etcd-release-tarball",
				Resource: "etcd-release-tarball",
				Passed: []string{
					"aws-acceptance-tests",
					"bosh-lite-acceptance-tests",
					"vsphere-acceptance-tests",
				},
			},
			{
				Name:     "cflinuxfs2-rootfs-release-tarball",
				Resource: "cflinuxfs2-rootfs-release-tarball",
				Passed: []string{
					"aws-acceptance-tests",
					"bosh-lite-acceptance-tests",
					"vsphere-acceptance-tests",
				},
			},
			{
				Name:     "vsphere-director-version",
				Resource: "vsphere-director-version",
				Passed: []string{
					"vsphere-acceptance-tests",
				},
			},
			{
				Name:     "aws-director-version",
				Resource: "aws-director-version",
				Passed: []string{
					"aws-acceptance-tests",
				},
			},
			{
				Name:     "bosh-lite-director-version",
				Resource: "bosh-lite-director-version",
				Passed: []string{
					"bosh-lite-acceptance-tests",
				},
			},
			{
				Name:     "vsphere-build-url",
				Resource: "vsphere-build-url",
				Passed: []string{
					"vsphere-acceptance-tests",
				},
			},
			{
				Name:     "aws-build-url",
				Resource: "aws-build-url",
				Passed: []string{
					"aws-acceptance-tests",
				},
			},
			{
				Name:     "bosh-lite-build-url",
				Resource: "bosh-lite-build-url",
				Passed: []string{
					"bosh-lite-acceptance-tests",
				},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"bosh-lite-stemcell":                "imported-r481v115766",
				"bosh-lite-build-url":               "imported-r678v147165",
				"vsphere-director-version":          "imported-r682v147119",
				"bosh-lite-director-version":        "imported-r681v147130",
				"bosh-lite":                         "imported-r480v140408",
				"runtime-ci":                        "imported-r494v147631",
				"garden-linux-release-tarball":      "imported-r477v132094",
				"etcd-release-tarball":              "imported-r478v142504",
				"diego-release-tarball":             "imported-r696v143482",
				"aws-stemcell":                      "imported-r482v146776",
				"vsphere-build-url":                 "imported-r679v147136",
				"aws-build-url":                     "imported-r677v147123",
				"aws-director-version":              "imported-r680v147110",
				"diego-cf-compatibility":            "imported-r475v147167",
				"cflinuxfs2-rootfs-release-tarball": "imported-r479v146681",
				"cf-release":                        "imported-r470v147078",
				"vsphere-stemcell":                  "imported-r484v146782",
			},
		},
	}),
)
