module NotFound.Model exposing (Model)

import Login.Login as Login
import Routes


type alias Model =
    Login.Model
        { route : Routes.Route
        , notFoundImgSrc : String
        }
