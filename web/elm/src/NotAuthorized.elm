port module NotAuthorized exposing (view)

import Html exposing (Html)
import Html.Attributes exposing (class, src, href)


view : Html msg
view =
    Html.div [ class "not-authorized" ]
        [ Html.img [] []
        , Html.div [ class "title" ] [ Html.text "401 Unauthorized" ]
        , Html.div [ class "reason" ] [ Html.text "You are not authorized to view" ]
        , Html.div [ class "reason" ] [ Html.text "the details of this pipeline" ]
        ]
