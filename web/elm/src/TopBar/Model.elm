module TopBar.Model exposing (Model)

import Dashboard.Group.Models exposing (Group)
import ScreenSize exposing (ScreenSize(..))


type alias Model r =
    { r
        | isUserMenuExpanded : Bool
        , groups : List Group
        , screenSize : ScreenSize
        , shiftDown : Bool
    }
