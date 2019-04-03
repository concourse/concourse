module Views.LoadingIndicator exposing (view)

import Html exposing (Html)
import Html.Attributes exposing (class, style)
import Message.Message exposing (Message)
import Views.Spinner as Spinner


view : Html Message
view =
    Html.div [ class "build-step" ]
        [ Html.div
            [ class "header"
            , style "display" "flex"
            ]
            [ Spinner.spinner
                { size = "14px"
                , margin = "7px"
                , hoverable = Nothing
                }
            , Html.h3 [] [ Html.text "loading..." ]
            ]
        ]
