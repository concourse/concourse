module LoadingIndicator exposing (view)

import Html exposing (Html)
import Html.Attributes exposing (class)


view : Html x
view =
    Html.div [ class "build-step" ]
        [ Html.div [ class "header" ]
            [ Html.i [ class "left fa fa-fw fa-spin fa-circle-o-notch" ] []
            , Html.h3 [] [ Html.text "loading" ]
            ]
        ]
