module TopBar.Model exposing
    ( Dropdown(..)
    , Model
    )

import Dashboard.Group.Models exposing (Group)
import ScreenSize exposing (ScreenSize(..))


type alias Model r =
    { r
        | isUserMenuExpanded : Bool
        , groups : List Group
        , dropdown : Dropdown
        , screenSize : ScreenSize
        , shiftDown : Bool
    }


type Dropdown
    = Hidden
    | Shown { selectedIdx : Maybe Int }
