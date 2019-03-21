module NotFound.Model exposing (Model)

import Routes
import TopBar.Model


type alias Model =
    TopBar.Model.Model
        { route : Routes.Route
        , notFoundImgSrc : String
        }
