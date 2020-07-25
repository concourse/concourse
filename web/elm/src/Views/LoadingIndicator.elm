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
            , class "loading-header"
            , style "display" "flex"
            ]
            [ Spinner.spinner
                { sizePx = 14
                , margin = "7px"
                }
            , Html.h3 [] [ Html.text "loading..." ]
            ]
        ]
