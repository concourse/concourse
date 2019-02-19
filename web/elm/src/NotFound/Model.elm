module NotFound.Model exposing (Model)

import TopBar.Model


type alias Model =
    { notFoundImgSrc : String
    , topBar : TopBar.Model.Model
    }
