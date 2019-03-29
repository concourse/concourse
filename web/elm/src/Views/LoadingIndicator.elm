module Views.LoadingIndicator exposing (view)

import Html exposing (Html)
import Html.Attributes exposing (class, style)
import Views.Spinner as Spinner


view : Html x
view =
    Html.div [ class "build-step" ]
        [ Html.div
            [ class "header"
            , style "display" "flex"
            ]
            [ Spinner.spinner { size = "14px", margin = "7px" }
            , Html.h3 [] [ Html.text "loading..." ]
            ]
        ]
