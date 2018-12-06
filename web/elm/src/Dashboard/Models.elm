module Dashboard.Models exposing (Pipeline)

import Concourse
import Concourse.PipelineStatus as PipelineStatus


type alias Pipeline =
    { id : Int
    , name : String
    , teamName : String
    , public : Bool
    , jobs : List Concourse.Job
    , resourceError : Bool
    , status : PipelineStatus.PipelineStatus
    }
