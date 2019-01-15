module LoadingIndicator exposing (view)

import Html exposing (Html)
import Html.Attributes exposing (class, style)
import Spinner


view : Html x
view =
    Html.div [ class "build-step" ]
        [ Html.div
            [ class "header"
            , style [ ( "display", "flex" ) ]
            ]
            [ Spinner.spinner "14px" [ style [ ( "margin", "7px" ) ] ]
            , Html.h3 [] [ Html.text "loading..." ]
            ]
        ]
