module NotFound.Model exposing (Model)

import TopBar.Model


type alias Model =
    TopBar.Model.Model { notFoundImgSrc : String }
