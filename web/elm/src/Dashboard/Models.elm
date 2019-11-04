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
        , groups : List Dashboard.Group.Models.Group
        , dropdown : Dropdown
        , highDensity : Bool
    }


type Dropdown
    = Hidden
    | Shown (Maybe Int)
