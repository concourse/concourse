module NewTopBar.Model exposing (Dropdown(..), Model, SearchBar(..))

import Concourse
import RemoteData
import Routes
import ScreenSize exposing (ScreenSize)
import UserState exposing (UserState)


type alias Model =
    { userState : UserState
    , isUserMenuExpanded : Bool
    , searchBar : SearchBar
    , teams : RemoteData.WebData (List Concourse.Team)
    , route : Routes.Route
    , screenSize : ScreenSize
    , highDensity : Bool
    }


type SearchBar
    = Gone
    | Minified
    | Visible { query : String, dropdown : Dropdown }


type Dropdown
    = Hidden
    | Shown { selectedIdx : Maybe Int }
