module Dashboard.Models exposing
    ( DashboardError(..)
    , DragState(..)
    , DropState(..)
    , Dropdown(..)
    , FooterModel
    , Model
    , SubState
    , tick
    )

import Concourse
import Dashboard.Group.Models
import Dict exposing (Dict)
import Login.Login as Login
import RemoteData
import Time


type DashboardError
    = Turbulence String


type alias Model =
    FooterModel
        (Login.Model
            { state : RemoteData.RemoteData DashboardError SubState
            , turbulencePath : String
            , pipelineRunningKeyframes : String
            , highDensity : Bool
            , query : String
            , pipelinesWithResourceErrors : Dict ( String, String ) Bool
            , existingJobs : List Concourse.Job
            }
        )


type alias SubState =
    { now : Time.Posix
    , dragState : DragState
    , dropState : DropState
    }


type DragState
    = NotDragging
    | Dragging Concourse.TeamName PipelineIndex


type DropState
    = NotDropping
    | Dropping PipelineIndex


type alias PipelineIndex =
    Int


tick : Time.Posix -> SubState -> SubState
tick now substate =
    { substate | now = now }


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
