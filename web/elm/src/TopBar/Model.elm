module TopBar.Model exposing
    ( Dropdown(..)
    , Model
    , PipelineState
    , isPaused
    )

import Concourse
import Dashboard.Group.Models exposing (Group)
import Routes
import ScreenSize exposing (ScreenSize(..))


type alias Model r =
    { r
        | isUserMenuExpanded : Bool
        , route : Routes.Route
        , groups : List Group
        , dropdown : Dropdown
        , screenSize : ScreenSize
        , shiftDown : Bool
    }


type Dropdown
    = Hidden
    | Shown { selectedIdx : Maybe Int }


type alias PipelineState =
    { pinnedResources : List ( String, Concourse.Version )
    , pipeline : Concourse.PipelineIdentifier
    , isPaused : Bool
    , isToggleHovered : Bool
    , isToggleLoading : Bool
    }


isPaused : Maybe PipelineState -> Bool
isPaused =
    Maybe.map .isPaused >> Maybe.withDefault False
