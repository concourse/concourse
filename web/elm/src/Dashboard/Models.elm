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
import Message.Message as Message
import RemoteData
import ScreenSize
import Time
import UserState


type DashboardError
    = Turbulence String


type alias Model =
    FooterModel
        (Login.Model
            { state : RemoteData.RemoteData DashboardError SubState
            , turbulencePath : String
            , pipelineRunningKeyframes : String
            , hovered : Maybe Message.Hoverable
            , userState : UserState.UserState
            , highDensity : Bool
            , query : String
            , instanceName : String
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
        , hovered : Maybe Message.Hoverable
        , screenSize : ScreenSize.ScreenSize
        , version : String
        , dropdown : Dropdown
        , highDensity : Bool
    }


type Dropdown
    = Hidden
    | Shown (Maybe Int)
