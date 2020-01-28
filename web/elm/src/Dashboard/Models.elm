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
import Login.Login as Login
import Time


type alias Model =
    FooterModel
        (Login.Model
            { showTurbulence : Bool
            , now : Maybe Time.Posix
            , highDensity : Bool
            , query : String
            , pipelinesWithResourceErrors : Dict ( String, String ) Bool
            , existingJobs : List Concourse.Job
            , pipelineLayers : Dict ( String, String ) (List (List Concourse.Job))
            , dragState : DragState
            , dropState : DropState
            , isJobsRequestFinished : Bool
            , isTeamsRequestFinished : Bool
            , isPipelinesRequestFinished : Bool
            , isResourcesRequestFinished : Bool
            , viewportWidth : Float
            , viewportHeight : Float
            , scrollTop : Float
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
        , teams : List Concourse.Team
        , groups : List Dashboard.Group.Models.Group
        , pipelines : List Dashboard.Group.Models.Pipeline
        , dropdown : Dropdown
        , highDensity : Bool
    }


type Dropdown
    = Hidden
    | Shown (Maybe Int)
