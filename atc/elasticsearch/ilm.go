package elasticsearch

// TODO: make configurable

const ilmPolicyName = "concourse-ilm-policy"

const ilmPolicyJSON = `{
  "policy": {
    "phases": {
      "hot": {
        "actions": {
          "rollover": {
            "max_size":"50gb",
            "max_age":"30d"
          },
          "set_priority": {
            "priority": 50
          }
        }
      },
      "warm": {
        "min_age": "7d",
        "actions": {
          "forcemerge": {
            "max_num_segments": 1
          },
          "shrink": {
            "number_of_shards": 1
          },
          "set_priority": {
            "priority": 25
          }
        }
      },
      "cold": {
        "min_age": "30d",
        "actions": {
          "set_priority": {
            "priority": 0
          },
          "freeze": {}
        }
      }
    }
  }
}`
