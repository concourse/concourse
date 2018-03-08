package atc_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
)

var _ = Describe("Plan", func() {
	It("returns a sanitized form of the plan", func() {
		plan := atc.Plan{
			ID: "0",
			Aggregate: &atc.AggregatePlan{
				atc.Plan{
					ID: "1",
					Aggregate: &atc.AggregatePlan{
						atc.Plan{
							ID: "2",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
					},
				},

				atc.Plan{
					ID: "3",
					Get: &atc.GetPlan{
						Type:     "type",
						Name:     "name",
						Resource: "resource",
						Source:   atc.Source{"some": "source"},
						Params:   atc.Params{"some": "params"},
						Version:  &atc.Version{"some": "version"},
						Tags:     atc.Tags{"tags"},
					},
				},

				atc.Plan{
					ID: "4",
					Put: &atc.PutPlan{
						Type:     "type",
						Name:     "name",
						Resource: "resource",
						Source:   atc.Source{"some": "source"},
						Params:   atc.Params{"some": "params"},
						Tags:     atc.Tags{"tags"},
					},
				},

				atc.Plan{
					ID: "5",
					Task: &atc.TaskPlan{
						Name:       "name",
						Privileged: true,
						Tags:       atc.Tags{"tags"},
						ConfigPath: "some/config/path.yml",
						Config: &atc.TaskConfig{
							Params: map[string]string{"some": "secret"},
						},
					},
				},

				atc.Plan{
					ID: "6",
					Ensure: &atc.EnsurePlan{
						Step: atc.Plan{
							ID: "7",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
						Next: atc.Plan{
							ID: "8",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
					},
				},

				atc.Plan{
					ID: "9",
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							ID: "10",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
						Next: atc.Plan{
							ID: "11",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
					},
				},

				atc.Plan{
					ID: "12",
					OnFailure: &atc.OnFailurePlan{
						Step: atc.Plan{
							ID: "13",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
						Next: atc.Plan{
							ID: "14",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
					},
				},

				atc.Plan{
					ID: "15",
					OnAbort: &atc.OnAbortPlan{
						Step: atc.Plan{
							ID: "16",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
						Next: atc.Plan{
							ID: "17",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
					},
				},

				atc.Plan{
					ID: "18",
					Try: &atc.TryPlan{
						Step: atc.Plan{
							ID: "19",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
					},
				},

				atc.Plan{
					ID: "20",
					Timeout: &atc.TimeoutPlan{
						Step: atc.Plan{
							ID: "21",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
						Duration: "lol",
					},
				},

				atc.Plan{
					ID: "22",
					Do: &atc.DoPlan{
						atc.Plan{
							ID: "23",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
					},
				},

				atc.Plan{
					ID: "24",
					Retry: &atc.RetryPlan{
						atc.Plan{
							ID: "25",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
						atc.Plan{
							ID: "26",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
						atc.Plan{
							ID: "27",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
					},
				},

				atc.Plan{
					ID: "28",
					OnAbort: &atc.OnAbortPlan{
						Step: atc.Plan{
							ID: "29",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
						Next: atc.Plan{
							ID: "30",
							Task: &atc.TaskPlan{
								Name:       "name",
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									Params: map[string]string{"some": "secret"},
								},
							},
						},
					},
				},
			},
		}

		json := plan.Public()
		Expect(json).ToNot(BeNil())
		Expect([]byte(*json)).To(MatchJSON(`{
  "id": "0",
  "aggregate": [
    {
      "id": "1",
      "aggregate": [
        {
          "id": "2",
          "task": {
            "name": "name",
            "privileged": false
          }
        }
      ]
    },
    {
      "id": "3",
      "get": {
        "type": "type",
        "name": "name",
        "resource": "resource",
        "version": {
          "some": "version"
        }
      }
    },
    {
      "id": "4",
      "put": {
        "type": "type",
        "name": "name",
        "resource": "resource"
      }
    },
    {
      "id": "5",
      "task": {
        "name": "name",
        "privileged": true
      }
    },
    {
      "id": "6",
      "ensure": {
        "step": {
          "id": "7",
          "task": {
            "name": "name",
            "privileged": false
          }
        },
        "ensure": {
          "id": "8",
          "task": {
            "name": "name",
            "privileged": false
          }
        }
      }
    },
    {
      "id": "9",
      "on_success": {
        "step": {
          "id": "10",
          "task": {
            "name": "name",
            "privileged": false
          }
        },
        "on_success": {
          "id": "11",
          "task": {
            "name": "name",
            "privileged": false
          }
        }
      }
    },
    {
      "id": "12",
      "on_failure": {
        "step": {
          "id": "13",
          "task": {
            "name": "name",
            "privileged": false
          }
        },
        "on_failure": {
          "id": "14",
          "task": {
            "name": "name",
            "privileged": false
          }
        }
      }
    },
    {
      "id": "15",
      "on_abort": {
        "step": {
          "id": "16",
          "task": {
            "name": "name",
            "privileged": false
          }
        },
        "on_abort": {
          "id": "17",
          "task": {
            "name": "name",
            "privileged": false
          }
        }
      }
    },
    {
      "id": "18",
      "try": {
        "step": {
          "id": "19",
          "task": {
            "name": "name",
            "privileged": false
          }
        }
      }
    },
    {
      "id": "20",
      "timeout": {
        "step": {
          "id": "21",
          "task": {
            "name": "name",
            "privileged": false
          }
        },
        "duration": "lol"
      }
    },
    {
      "id": "22",
      "do": [
        {
          "id": "23",
          "task": {
            "name": "name",
            "privileged": false
          }
        }
      ]
    },
    {
      "id": "24",
      "retry": [
        {
          "id": "25",
          "task": {
            "name": "name",
            "privileged": false
          }
        },
        {
          "id": "26",
          "task": {
            "name": "name",
            "privileged": false
          }
        },
        {
          "id": "27",
          "task": {
            "name": "name",
            "privileged": false
          }
        }
      ]
    },
    {
      "id": "28",
      "on_abort": {
	    "step": {
          "id": "29",
          "task": {
            "name": "name",
            "privileged": false
          }
	    },
        "on_abort": {
          "id": "30",
          "task": {
            "name": "name",
            "privileged": false
          }
	    }
      }
    }
  ]
}
`))
	})
})
