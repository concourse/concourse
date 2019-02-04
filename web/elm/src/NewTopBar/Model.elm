module NewTopBar.Model exposing
    ( Dropdown(..)
    , MiddleSection(..)
    , Model
    )

import Concourse
import RemoteData
import Routes
import ScreenSize exposing (ScreenSize)
import UserState exposing (UserState)


type alias Model =
    { userState : UserState
    , isUserMenuExpanded : Bool
    , middleSection : MiddleSection
    , teams : RemoteData.WebData (List Concourse.Team)
    , screenSize : ScreenSize
    , highDensity : Bool
    }



-- The Route in middle section should always be a pipeline, build, resource, or job, but that's hard to demonstrate statically


type MiddleSection
    = Breadcrumbs Routes.Route
    | MinifiedSearch
    | SearchBar { query : String, dropdown : Dropdown }
    | Empty


type Dropdown
    = Hidden
    | Shown { selectedIdx : Maybe Int }
