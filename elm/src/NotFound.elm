port module NotFound exposing (Model, view)

import Html exposing (Html)
import Html.Attributes exposing (class, src, href)


type alias Model =
    { notFoundImgSrc : String
    }


view : Model -> Html msg
view model =
    Html.div [ class "notfound" ]
        [ Html.div [ class "title" ] [ Html.text "404" ]
        , Html.div [ class "reason" ] [ Html.text "this page was not found" ]
        , Html.img [ src model.notFoundImgSrc ] []
        , Html.div [ class "help-message" ] [ Html.text "Not to worry, you can head", Html.br [] [], Html.text "back to the ", Html.a [ href "/" ] [ Html.text "home page" ] ]
        ]
