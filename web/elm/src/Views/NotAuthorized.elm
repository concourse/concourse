module Views.NotAuthorized exposing (view)

import Assets
import Html exposing (Html)
import Html.Attributes exposing (class, src)


view : Html msg
view =
    Html.div [ class "not-authorized" ]
        [ Html.img
            [ src <| Assets.toString Assets.PassportOfficerIcon ]
            []
        , Html.div
            [ class "title" ]
            [ Html.text "401 Unauthorized" ]
        , Html.div
            [ class "reason" ]
            [ Html.text "You are not authorized to view" ]
        , Html.div
            [ class "reason" ]
            [ Html.text "the details of this pipeline" ]
        ]
