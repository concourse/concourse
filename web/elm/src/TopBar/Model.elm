module TopBar.Model exposing
    ( Dropdown(..)
    , Model
    )

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
