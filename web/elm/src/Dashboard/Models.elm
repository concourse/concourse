module Dashboard.Models exposing
    ( DashboardError(..)
    , DragState(..)
    , DropState(..)
    , FooterModel
    , Group
    , Model
    , Pipeline
    , SubState
    , tick
    )

import Concourse
import Concourse.PipelineStatus as PipelineStatus
import Dashboard.Group.Tag as Tag
import Message.Message as Message
import RemoteData
import ScreenSize
import Time exposing (Time)
import TopBar.Model
import UserState


type DashboardError
    = Turbulence String


type alias Model =
    FooterModel
        (TopBar.Model.Model
            { state : RemoteData.RemoteData DashboardError SubState
            , turbulencePath : String
            , pipelineRunningKeyframes : String
            , hovered : Maybe Message.Hoverable
            , userState : UserState.UserState
            }
        )


type alias SubState =
    { now : Time
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


tick : Time.Time -> SubState -> SubState
tick now substate =
    { substate | now = now }


type alias Pipeline =
    { id : Int
    , name : String
    , teamName : String
    , public : Bool
    , jobs : List Concourse.Job
    , resourceError : Bool
    , status : PipelineStatus.PipelineStatus
    }


type alias FooterModel r =
    { r
        | hideFooter : Bool
        , hideFooterCounter : Int
        , showHelp : Bool
        , groups : List Group
        , hovered : Maybe Message.Hoverable
        , screenSize : ScreenSize.ScreenSize
        , version : String
        , highDensity : Bool
    }


type alias Group =
    { pipelines : List Pipeline
    , teamName : String
    , tag : Maybe Tag.Tag
    }
