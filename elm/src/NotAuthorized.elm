port module NotAuthorized exposing (view)

import Html exposing (Html)
import Html.Attributes exposing (class, src, href)


view : Html msg
view =
    Html.div [ class "not-authorized" ]
        [ Html.div [ class "title" ] [ Html.text "401" ]
        , Html.div [ class "reason" ] [ Html.text "You are not authorized to view this resource" ]
        ]
