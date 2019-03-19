module Dashboard.Models exposing
    ( DashboardError(..)
    , DragState(..)
    , DropState(..)
    , FooterModel
    , Model
    , SubState
    , tick
    )

import Concourse
import Dashboard.Group.Models
import Message.Message as Message
import RemoteData
import Routes
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


type alias FooterModel r =
    { r
        | hideFooter : Bool
        , hideFooterCounter : Int
        , showHelp : Bool
        , groups : List Dashboard.Group.Models.Group
        , hovered : Maybe Message.Hoverable
        , screenSize : ScreenSize.ScreenSize
        , version : String
        , route : Routes.Route
        , shiftDown : Bool
        , dropdown : TopBar.Model.Dropdown
    }
