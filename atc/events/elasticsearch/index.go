package elasticsearch

// TODO: make configurable

const indexTemplateName = "concourse-build-events"

const indexPatternPrefix = "concourse-build-events"

const indexTemplateJSON = `{
  "index_patterns": "` + indexPatternPrefix + `-*", 
  "settings":{
    "index.number_of_shards": 1,
    "index.lifecycle.name": "` + ilmPolicyName + `",
    "index.lifecycle.rollover_alias": "` + indexPatternPrefix + `"
  },
  "mappings":{
    "properties":{
      "build_id":{
        "type":"integer"
      },
      "build_name":{
        "type": "keyword",
        "ignore_above": 256
      },
      "job_id":{
        "type":"integer"
      },
      "job_name":{
        "type": "keyword",
        "ignore_above": 256
      },
      "pipeline_id":{
        "type":"integer"
      },
      "pipeline_name":{
        "type": "keyword",
        "ignore_above": 256
      },
      "team_id":{
        "type":"integer"
      },
      "team_name":{
        "type": "keyword",
        "ignore_above": 256
      },
      "event":{
        "type": "keyword",
        "ignore_above": 256
      },
      "version":{
        "type": "keyword",
        "ignore_above": 256
      },
      "tiebreak":{
        "type":"long"
      },
      "data":{
        "properties": {
          "time": {
            "type": "date",
            "format": "epoch_second"
          },
          "origin": {
            "properties": {
              "id": {
                "type": "keyword",
                "ignore_above": 256
              },
              "source": {
                "type": "keyword",
                "ignore_above": 256
              }
            }
          },
          "payload": {
            "type": "text"
          }
        }
      }
    }
  }
}`

const initialIndexName = indexPatternPrefix + "-000001"

const initialIndexJSON = `
{
  "aliases": {
    "` + indexPatternPrefix + `": {
      "is_write_index": true
    }
  }
}
`
