port module NotFound exposing (Model, view)

import Html exposing (Html)
import Html.Attributes exposing (class, src)

type alias Model =
    { notFoundImgSrc: String
    }

view : Model -> Html msg
view model =
    Html.div [ class "display-in-middle"]
        [ Html.div [ class "title" ] [Html.text "404"]
        , Html.div [ class "reason" ] [ Html.text "this page was not found" ]
        , Html.img [ src model.notFoundImgSrc, class "404-image" ] []
        , Html.div [ class "help-message" ] [ Html.text "not to worry, you can head back to the home page" ]
        ]
