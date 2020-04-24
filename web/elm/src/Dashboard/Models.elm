module Dashboard.Models exposing
    ( DragState(..)
    , DropState(..)
    , Dropdown(..)
    , FooterModel
    , Model
    )

import Concourse
import Dashboard.Group.Models
import Dict exposing (Dict)
import FetchResult exposing (FetchResult)
import Login.Login as Login
import Message.Effects exposing (Effect)
import Set exposing (Set)
import Time


type alias Model =
    FooterModel
        (Login.Model
            { now : Maybe Time.Posix
            , highDensity : Bool
            , query : String
            , pipelinesWithResourceErrors : Dict ( String, String ) Bool
            , jobs : FetchResult (Dict ( String, String, String ) Concourse.Job)
            , pipelineLayers : Dict ( String, String ) (List (List Concourse.JobIdentifier))
            , teams : FetchResult (List Concourse.Team)
            , dragState : DragState
            , dropState : DropState
            , isJobsRequestFinished : Bool
            , isTeamsRequestFinished : Bool
            , isPipelinesRequestFinished : Bool
            , isResourcesRequestFinished : Bool
            , isJobsErroring : Bool
            , isTeamsErroring : Bool
            , isResourcesErroring : Bool
            , isPipelinesErroring : Bool
            , viewportWidth : Float
            , viewportHeight : Float
            , scrollTop : Float
            , pipelineJobs : Dict ( String, String ) (List Concourse.JobIdentifier)
            }
        )


type DragState
    = NotDragging
    | Dragging Concourse.TeamName PipelineIndex


type DropState
    = NotDropping
    | Dropping PipelineIndex
    | DroppingWhileApiRequestInFlight Concourse.TeamName


type alias PipelineIndex =
    Int


type alias FooterModel r =
    { r
        | hideFooter : Bool
        , hideFooterCounter : Int
        , showHelp : Bool
        , pipelines : FetchResult (List Dashboard.Group.Models.Pipeline)
        , dropdown : Dropdown
        , highDensity : Bool
    }


type Dropdown
    = Hidden
    | Shown (Maybe Int)
