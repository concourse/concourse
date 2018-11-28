module Dashboard.APIData exposing (APIData)

import Concourse


type alias APIData =
    { teams : List Concourse.Team
    , pipelines : List Concourse.Pipeline
    , jobs : List Concourse.Job
    , resources : List Concourse.Resource
    , version : String
    }
